package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"
)

func runReplication(args []string) {
	if len(args) == 0 {
		fmt.Println(`Usage: vaults3-cli replication <subcommand>

Subcommands:
  status               Show replication peer status
  queue                Show replication queue`)
		os.Exit(1)
	}

	requireCreds()

	switch args[0] {
	case "status":
		replicationStatus()
	case "queue":
		replicationQueue()
	default:
		fatal("unknown replication subcommand: " + args[0])
	}
}

func replicationStatus() {
	resp, err := apiRequest("GET", "/replication/status", nil)
	if err != nil {
		fatal(err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		fatal(fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body)))
	}

	var statuses []struct {
		Peer         string `json:"peer"`
		QueueDepth   int    `json:"queue_depth"`
		LastSyncTime int64  `json:"last_sync_time"`
		LastError    string `json:"last_error"`
		TotalSynced  int64  `json:"total_synced"`
		TotalFailed  int64  `json:"total_failed"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&statuses); err != nil {
		fatal("parse response: " + err.Error())
	}

	if len(statuses) == 0 {
		fmt.Println("No replication peers configured.")
		return
	}

	headers := []string{"PEER", "QUEUE", "SYNCED", "FAILED", "LAST SYNC", "LAST ERROR"}
	var rows [][]string
	for _, s := range statuses {
		lastSync := "never"
		if s.LastSyncTime > 0 {
			lastSync = time.Unix(s.LastSyncTime, 0).Format("2006-01-02 15:04:05")
		}
		lastErr := "-"
		if s.LastError != "" {
			lastErr = s.LastError
			if len(lastErr) > 40 {
				lastErr = lastErr[:40] + "..."
			}
		}
		rows = append(rows, []string{
			s.Peer,
			strconv.Itoa(s.QueueDepth),
			strconv.FormatInt(s.TotalSynced, 10),
			strconv.FormatInt(s.TotalFailed, 10),
			lastSync,
			lastErr,
		})
	}
	printTable(headers, rows)
}

func replicationQueue() {
	resp, err := apiRequest("GET", "/replication/queue?limit=20", nil)
	if err != nil {
		fatal(err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		fatal(fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body)))
	}

	var events []struct {
		ID         uint64 `json:"id"`
		Type       string `json:"type"`
		Bucket     string `json:"bucket"`
		Key        string `json:"key"`
		Peer       string `json:"peer"`
		RetryCount int    `json:"retry_count"`
		CreatedAt  int64  `json:"created_at"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&events); err != nil {
		fatal("parse response: " + err.Error())
	}

	if len(events) == 0 {
		fmt.Println("Replication queue is empty.")
		return
	}

	headers := []string{"ID", "TYPE", "BUCKET", "KEY", "PEER", "RETRIES", "CREATED"}
	var rows [][]string
	for _, e := range events {
		created := time.Unix(e.CreatedAt, 0).Format("2006-01-02 15:04:05")
		rows = append(rows, []string{
			strconv.FormatUint(e.ID, 10),
			e.Type,
			e.Bucket,
			e.Key,
			e.Peer,
			strconv.Itoa(e.RetryCount),
			created,
		})
	}
	printTable(headers, rows)
}
