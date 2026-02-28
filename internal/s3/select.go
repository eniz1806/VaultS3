package s3

import (
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/parquet-go/parquet-go"
)

const maxSelectSize int64 = 256 * 1024 * 1024

// SelectObjectContent handles POST /{bucket}/{key}?select&select-type=2.
func (h *ObjectHandler) SelectObjectContent(w http.ResponseWriter, r *http.Request, bucket, key string) {
	if !h.store.BucketExists(bucket) {
		writeS3Error(w, "NoSuchBucket", "Bucket does not exist", http.StatusNotFound)
		return
	}

	// Parse the XML request body (limit to 64KB)
	var req selectRequest
	if err := xml.NewDecoder(io.LimitReader(r.Body, 64*1024)).Decode(&req); err != nil {
		writeS3Error(w, "MalformedXML", "Could not parse SelectObjectContentRequest", http.StatusBadRequest)
		return
	}

	if req.ExpressionType != "SQL" {
		writeS3Error(w, "InvalidArgument", "Only SQL expression type is supported", http.StatusBadRequest)
		return
	}

	// Parse SQL expression
	query, err := parseSQL(req.Expression)
	if err != nil {
		writeS3Error(w, "InvalidArgument", fmt.Sprintf("Invalid SQL: %s", err), http.StatusBadRequest)
		return
	}

	// Reject objects larger than 256MB for S3 Select to prevent OOM
	meta, _ := h.store.GetObjectMeta(bucket, key)
	if meta != nil && meta.Size > maxSelectSize {
		writeS3Error(w, "InvalidArgument", "Object too large for S3 Select (max 256MB)", http.StatusBadRequest)
		return
	}

	// Read the object (handle versioned storage)
	var reader io.ReadCloser
	if meta != nil && meta.VersionID != "" {
		r, _, err := h.engine.GetObjectVersion(bucket, key, meta.VersionID)
		if err != nil {
			writeS3Error(w, "NoSuchKey", "Object not found", http.StatusNotFound)
			return
		}
		reader = r
	} else {
		r, _, err := h.engine.GetObject(bucket, key)
		if err != nil {
			writeS3Error(w, "NoSuchKey", "Object not found", http.StatusNotFound)
			return
		}
		reader = r
	}
	defer reader.Close()

	// Decompress if needed
	var dataReader io.Reader = reader
	switch strings.ToUpper(req.InputSerialization.CompressionType) {
	case "GZIP":
		gz, err := gzip.NewReader(reader)
		if err != nil {
			writeS3Error(w, "InvalidArgument", "Failed to decompress GZIP input", http.StatusBadRequest)
			return
		}
		defer gz.Close()
		dataReader = gz
	case "BZIP2":
		dataReader = bzip2.NewReader(reader)
	case "NONE", "":
		// no decompression
	default:
		writeS3Error(w, "InvalidArgument", fmt.Sprintf("Unsupported CompressionType: %s", req.InputSerialization.CompressionType), http.StatusBadRequest)
		return
	}

	// Determine input format
	var records []map[string]string
	if req.InputSerialization.CSV != nil {
		records, err = parseCSVInput(dataReader, req.InputSerialization.CSV)
	} else if req.InputSerialization.JSON != nil {
		records, err = parseJSONInput(dataReader, req.InputSerialization.JSON)
	} else if req.InputSerialization.Parquet != nil {
		records, err = parseParquetInput(dataReader)
	} else {
		writeS3Error(w, "InvalidArgument", "InputSerialization must specify CSV, JSON, or Parquet", http.StatusBadRequest)
		return
	}
	if err != nil {
		writeS3Error(w, "InternalError", fmt.Sprintf("Failed to parse input: %s", err), http.StatusInternalServerError)
		return
	}

	// Execute query
	results := executeQuery(query, records)

	// Write output
	if req.OutputSerialization.JSON != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		enc := json.NewEncoder(w)
		for _, rec := range results {
			enc.Encode(rec)
		}
	} else {
		// Default to CSV output
		w.Header().Set("Content-Type", "text/csv")
		w.WriteHeader(http.StatusOK)
		cw := csv.NewWriter(w)
		delim := ','
		if req.OutputSerialization.CSV != nil && req.OutputSerialization.CSV.FieldDelimiter != "" {
			delim = rune(req.OutputSerialization.CSV.FieldDelimiter[0])
		}
		cw.Comma = delim

		// Write header if we have results
		if len(results) > 0 {
			var headers []string
			for k := range results[0] {
				headers = append(headers, k)
			}
			// Sort headers for deterministic output
			sortStrings(headers)
			cw.Write(headers)

			for _, rec := range results {
				var row []string
				for _, h := range headers {
					row = append(row, rec[h])
				}
				cw.Write(row)
			}
		}
		cw.Flush()
	}
}

// XML request types

type selectRequest struct {
	XMLName             xml.Name            `xml:"SelectObjectContentRequest"`
	Expression          string              `xml:"Expression"`
	ExpressionType      string              `xml:"ExpressionType"`
	InputSerialization  inputSerialization  `xml:"InputSerialization"`
	OutputSerialization outputSerialization `xml:"OutputSerialization"`
}

type inputSerialization struct {
	CompressionType string        `xml:"CompressionType"` // NONE, GZIP, BZIP2
	CSV             *csvInput     `xml:"CSV"`
	JSON            *jsonInput    `xml:"JSON"`
	Parquet         *parquetInput `xml:"Parquet"`
}

type parquetInput struct{}

type csvInput struct {
	FileHeaderInfo  string `xml:"FileHeaderInfo"` // USE, IGNORE, NONE
	FieldDelimiter  string `xml:"FieldDelimiter"`
	RecordDelimiter string `xml:"RecordDelimiter"`
	QuoteCharacter  string `xml:"QuoteCharacter"`
}

type jsonInput struct {
	Type string `xml:"Type"` // DOCUMENT or LINES
}

type outputSerialization struct {
	CSV  *csvOutput  `xml:"CSV"`
	JSON *jsonOutput `xml:"JSON"`
}

type csvOutput struct {
	FieldDelimiter  string `xml:"FieldDelimiter"`
	RecordDelimiter string `xml:"RecordDelimiter"`
}

type jsonOutput struct {
	RecordDelimiter string `xml:"RecordDelimiter"`
}

// SQL parsing

type selectQuery struct {
	columns    []string // "*" or list of column names
	conditions []condition
	limit      int
}

type condition struct {
	column  string
	op      string // =, !=, <, >, <=, >=, LIKE, IS
	value   string
	logic   string // AND, OR (for chaining with next condition)
	isNull  bool   // for IS NULL / IS NOT NULL
	notNull bool
}

var sqlSelectRe = regexp.MustCompile(`(?i)^SELECT\s+(.+?)\s+FROM\s+s3object(?:\s+(?:s|s3object))?\s*(.*)$`)
var whereRe = regexp.MustCompile(`(?i)^WHERE\s+(.+)$`)
var limitRe = regexp.MustCompile(`(?i)\s+LIMIT\s+(\d+)\s*$`)

func parseSQL(expr string) (*selectQuery, error) {
	expr = strings.TrimSpace(expr)
	q := &selectQuery{limit: -1}

	// Extract LIMIT clause first
	if matches := limitRe.FindStringSubmatch(expr); len(matches) == 2 {
		n, err := strconv.Atoi(matches[1])
		if err != nil {
			return nil, fmt.Errorf("invalid LIMIT value")
		}
		q.limit = n
		expr = expr[:len(expr)-len(matches[0])]
	}

	// Match SELECT ... FROM s3object ...
	matches := sqlSelectRe.FindStringSubmatch(expr)
	if len(matches) < 3 {
		return nil, fmt.Errorf("expected SELECT ... FROM s3object")
	}

	// Parse columns
	colsPart := strings.TrimSpace(matches[1])
	if colsPart == "*" {
		q.columns = []string{"*"}
	} else {
		cols := strings.Split(colsPart, ",")
		for _, c := range cols {
			c = strings.TrimSpace(c)
			// Strip s3object. or s. prefix
			c = stripS3Prefix(c)
			if c != "" {
				q.columns = append(q.columns, c)
			}
		}
	}

	// Parse WHERE clause
	rest := strings.TrimSpace(matches[2])
	if rest != "" {
		wm := whereRe.FindStringSubmatch(rest)
		if len(wm) < 2 {
			return nil, fmt.Errorf("unexpected clause: %s", rest)
		}
		conds, err := parseConditions(wm[1])
		if err != nil {
			return nil, err
		}
		q.conditions = conds
	}

	return q, nil
}

func stripS3Prefix(col string) string {
	prefixes := []string{"s3object.", "s.", "s3object[", "s["}
	lower := strings.ToLower(col)
	for _, p := range prefixes {
		if strings.HasPrefix(lower, p) {
			col = col[len(p):]
			// Handle bracket notation: s3object['name'] or s3object["name"]
			col = strings.Trim(col, "[]'\"")
			return col
		}
	}
	return col
}

var condRe = regexp.MustCompile(`(?i)^(.+?)\s+(=|!=|<>|<=|>=|<|>|LIKE|NOT\s+LIKE|IS\s+NOT|IS)\s+(.+)$`)

func parseConditions(expr string) ([]condition, error) {
	var conditions []condition

	// Split on AND/OR (simple split, no nested parens support)
	parts := splitLogical(expr)

	for _, part := range parts {
		p := strings.TrimSpace(part.expr)
		if p == "" {
			continue
		}

		m := condRe.FindStringSubmatch(p)
		if len(m) < 4 {
			return nil, fmt.Errorf("cannot parse condition: %s", p)
		}

		col := strings.TrimSpace(m[1])
		col = stripS3Prefix(col)
		op := strings.ToUpper(strings.TrimSpace(m[2]))
		val := strings.TrimSpace(m[3])

		c := condition{
			column: col,
			op:     op,
			logic:  part.logic,
		}

		if op == "IS NOT" {
			if strings.ToUpper(val) == "NULL" {
				c.notNull = true
			}
		} else if op == "IS" {
			if strings.ToUpper(val) == "NULL" {
				c.isNull = true
			}
		} else {
			// Strip quotes from value
			val = strings.Trim(val, "'\"")
			c.value = val
		}

		conditions = append(conditions, c)
	}

	return conditions, nil
}

type logicalPart struct {
	expr  string
	logic string // "", "AND", "OR"
}

func splitLogical(expr string) []logicalPart {
	var parts []logicalPart
	upper := strings.ToUpper(expr)

	// Simple split on AND/OR boundaries
	var current strings.Builder
	words := tokenize(expr)
	logic := ""

	for i := 0; i < len(words); i++ {
		w := strings.ToUpper(words[i])
		if w == "AND" || w == "OR" {
			if current.Len() > 0 {
				parts = append(parts, logicalPart{expr: strings.TrimSpace(current.String()), logic: logic})
				current.Reset()
			}
			logic = w
			continue
		}
		_ = upper // suppress unused
		if current.Len() > 0 {
			current.WriteByte(' ')
		}
		current.WriteString(words[i])
	}
	if current.Len() > 0 {
		parts = append(parts, logicalPart{expr: strings.TrimSpace(current.String()), logic: logic})
	}

	return parts
}

func tokenize(expr string) []string {
	var tokens []string
	var current strings.Builder
	inQuote := byte(0)

	for i := 0; i < len(expr); i++ {
		ch := expr[i]
		if inQuote != 0 {
			current.WriteByte(ch)
			if ch == inQuote {
				inQuote = 0
			}
			continue
		}
		if ch == '\'' || ch == '"' {
			current.WriteByte(ch)
			inQuote = ch
			continue
		}
		if ch == ' ' || ch == '\t' || ch == '\n' {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			continue
		}
		current.WriteByte(ch)
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}
	return tokens
}

// Input parsers

func parseCSVInput(reader io.Reader, cfg *csvInput) ([]map[string]string, error) {
	cr := csv.NewReader(reader)
	if cfg.FieldDelimiter != "" {
		cr.Comma = rune(cfg.FieldDelimiter[0])
	}
	cr.LazyQuotes = true
	cr.FieldsPerRecord = -1

	allRows, err := cr.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(allRows) == 0 {
		return nil, nil
	}

	// Determine headers
	var headers []string
	startRow := 0
	headerInfo := strings.ToUpper(cfg.FileHeaderInfo)
	if headerInfo == "" {
		headerInfo = "NONE"
	}

	switch headerInfo {
	case "USE":
		headers = allRows[0]
		startRow = 1
	case "IGNORE":
		startRow = 1
		for i := range allRows[0] {
			headers = append(headers, fmt.Sprintf("_%d", i+1))
		}
	default: // NONE
		if len(allRows) > 0 {
			for i := range allRows[0] {
				headers = append(headers, fmt.Sprintf("_%d", i+1))
			}
		}
	}

	var records []map[string]string
	for _, row := range allRows[startRow:] {
		rec := make(map[string]string)
		for i, val := range row {
			if i < len(headers) {
				rec[headers[i]] = val
			}
		}
		records = append(records, rec)
	}

	return records, nil
}

// maxSelectRecords caps the number of records parsed to prevent memory exhaustion.
const maxSelectRecords = 1_000_000

func parseJSONInput(reader io.Reader, cfg *jsonInput) ([]map[string]string, error) {
	// Limit read to maxSelectSize (256MB) to prevent OOM
	data, err := io.ReadAll(io.LimitReader(reader, maxSelectSize+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxSelectSize {
		return nil, fmt.Errorf("JSON input exceeds maximum size")
	}

	jsonType := strings.ToUpper(cfg.Type)
	if jsonType == "" {
		jsonType = "LINES"
	}

	var rawRecords []map[string]interface{}

	switch jsonType {
	case "DOCUMENT":
		// Use json.Decoder with depth check via limited-size data
		if err := json.Unmarshal(data, &rawRecords); err != nil {
			// Try single object
			var single map[string]interface{}
			if err2 := json.Unmarshal(data, &single); err2 != nil {
				return nil, fmt.Errorf("cannot parse JSON document: %v", err)
			}
			rawRecords = []map[string]interface{}{single}
		}
		if len(rawRecords) > maxSelectRecords {
			return nil, fmt.Errorf("too many records (max %d)", maxSelectRecords)
		}
	default: // LINES
		lines := strings.Split(strings.TrimSpace(string(data)), "\n")
		if len(lines) > maxSelectRecords {
			return nil, fmt.Errorf("too many lines (max %d)", maxSelectRecords)
		}
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			var rec map[string]interface{}
			if err := json.Unmarshal([]byte(line), &rec); err != nil {
				return nil, fmt.Errorf("cannot parse JSON line: %v", err)
			}
			rawRecords = append(rawRecords, rec)
		}
	}

	// Convert to map[string]string
	var records []map[string]string
	for _, raw := range rawRecords {
		rec := make(map[string]string)
		for k, v := range raw {
			rec[k] = fmt.Sprintf("%v", v)
		}
		records = append(records, rec)
	}

	return records, nil
}

// Query execution

func executeQuery(q *selectQuery, records []map[string]string) []map[string]string {
	var results []map[string]string

	for _, rec := range records {
		if !matchesConditions(rec, q.conditions) {
			continue
		}

		// Project columns
		if len(q.columns) == 1 && q.columns[0] == "*" {
			results = append(results, rec)
		} else {
			projected := make(map[string]string)
			for _, col := range q.columns {
				projected[col] = rec[col]
			}
			results = append(results, projected)
		}

		if q.limit > 0 && len(results) >= q.limit {
			break
		}
	}

	return results
}

func matchesConditions(rec map[string]string, conditions []condition) bool {
	if len(conditions) == 0 {
		return true
	}

	result := evaluateCondition(rec, conditions[0])

	for i := 1; i < len(conditions); i++ {
		c := conditions[i]
		val := evaluateCondition(rec, c)

		switch c.logic {
		case "OR":
			result = result || val
		default: // AND
			result = result && val
		}
	}

	return result
}

func evaluateCondition(rec map[string]string, c condition) bool {
	val, exists := rec[c.column]

	if c.isNull {
		return !exists || val == ""
	}
	if c.notNull {
		return exists && val != ""
	}

	switch c.op {
	case "=":
		return val == c.value
	case "!=", "<>":
		return val != c.value
	case "<":
		return compareValues(val, c.value) < 0
	case ">":
		return compareValues(val, c.value) > 0
	case "<=":
		return compareValues(val, c.value) <= 0
	case ">=":
		return compareValues(val, c.value) >= 0
	case "LIKE":
		return matchLike(val, c.value)
	case "NOT LIKE":
		return !matchLike(val, c.value)
	}

	return false
}

func compareValues(a, b string) int {
	// Try numeric comparison first
	af, errA := strconv.ParseFloat(a, 64)
	bf, errB := strconv.ParseFloat(b, 64)
	if errA == nil && errB == nil {
		if af < bf {
			return -1
		}
		if af > bf {
			return 1
		}
		return 0
	}
	return strings.Compare(a, b)
}

func matchLike(val, pattern string) bool {
	// Iterative DP-based LIKE matching (avoids exponential backtracking)
	// % = match any sequence, _ = match single character
	// Case-insensitive
	v := strings.ToLower(val)
	p := strings.ToLower(pattern)

	// Cap pattern complexity to prevent abuse
	if len(p) > 256 {
		return false
	}

	vLen := len(v)
	pLen := len(p)

	// dp[j] = true means v[:i] matches p[:j]
	dp := make([]bool, pLen+1)
	dp[0] = true

	// Initialize: leading % patterns match empty string
	for j := 1; j <= pLen; j++ {
		if p[j-1] == '%' {
			dp[j] = dp[j-1]
		} else {
			break
		}
	}

	for i := 1; i <= vLen; i++ {
		prev := dp[0]
		dp[0] = false
		for j := 1; j <= pLen; j++ {
			temp := dp[j]
			switch p[j-1] {
			case '%':
				// % matches zero (dp[j-1]) or more (dp[j] from previous row) characters
				dp[j] = dp[j-1] || dp[j]
			case '_':
				dp[j] = prev
			default:
				dp[j] = prev && v[i-1] == p[j-1]
			}
			prev = temp
		}
	}

	return dp[pLen]
}

func sortStrings(s []string) {
	for i := 0; i < len(s); i++ {
		for j := i + 1; j < len(s); j++ {
			if s[j] < s[i] {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
}

// parseParquetInput reads a Parquet file and converts it to records.
func parseParquetInput(reader io.Reader) ([]map[string]string, error) {
	// Parquet requires random access, so read all data into memory
	data, err := io.ReadAll(io.LimitReader(reader, maxSelectSize+1))
	if err != nil {
		return nil, fmt.Errorf("read parquet data: %w", err)
	}
	if int64(len(data)) > maxSelectSize {
		return nil, fmt.Errorf("Parquet input exceeds maximum size")
	}

	file, err := parquet.OpenFile(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("open parquet file: %w", err)
	}

	schema := file.Schema()
	fields := schema.Fields()
	colNames := make([]string, len(fields))
	for i, f := range fields {
		colNames[i] = f.Name()
	}

	var records []map[string]string
	for _, rg := range file.RowGroups() {
		rows := rg.NumRows()
		if int64(len(records))+rows > int64(maxSelectRecords) {
			return nil, fmt.Errorf("too many records (max %d)", maxSelectRecords)
		}

		rowBuf := make([]parquet.Row, 256)
		rowReader := rg.Rows()
		defer rowReader.Close()

		for {
			n, err := rowReader.ReadRows(rowBuf)
			for i := 0; i < n; i++ {
				rec := make(map[string]string, len(colNames))
				for _, v := range rowBuf[i] {
					col := v.Column()
					if col >= 0 && col < len(colNames) {
						rec[colNames[col]] = fmt.Sprintf("%v", v)
					}
				}
				records = append(records, rec)
			}
			if err != nil {
				break
			}
		}
	}

	return records, nil
}
