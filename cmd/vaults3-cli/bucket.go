package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"time"
)

func runBucket(args []string) {
	if len(args) == 0 {
		fmt.Println(`Usage: vaults3-cli bucket <subcommand>

Subcommands:
  list                List all buckets
  create <name>       Create a bucket
  delete <name>       Delete a bucket
  info <name>         Show bucket details`)
		os.Exit(1)
	}

	requireCreds()

	switch args[0] {
	case "list", "ls":
		bucketList()
	case "create":
		if len(args) < 2 {
			fatal("bucket create requires a bucket name")
		}
		bucketCreate(args[1])
	case "delete", "rm":
		if len(args) < 2 {
			fatal("bucket delete requires a bucket name")
		}
		bucketDelete(args[1])
	case "info":
		if len(args) < 2 {
			fatal("bucket info requires a bucket name")
		}
		bucketInfo(args[1])
	default:
		fatal("unknown bucket subcommand: " + args[0])
	}
}

func bucketList() {
	resp, err := s3Request("GET", "/", nil)
	if err != nil {
		fatal(err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		fatal(fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body)))
	}

	var result struct {
		XMLName xml.Name `xml:"ListAllMyBucketsResult"`
		Buckets struct {
			Bucket []struct {
				Name         string `xml:"Name"`
				CreationDate string `xml:"CreationDate"`
			} `xml:"Bucket"`
		} `xml:"Buckets"`
	}
	if err := xml.NewDecoder(resp.Body).Decode(&result); err != nil {
		fatal("parse response: " + err.Error())
	}

	if len(result.Buckets.Bucket) == 0 {
		fmt.Println("No buckets found.")
		return
	}

	headers := []string{"NAME", "CREATED"}
	var rows [][]string
	for _, b := range result.Buckets.Bucket {
		t, _ := time.Parse(time.RFC3339Nano, b.CreationDate)
		rows = append(rows, []string{b.Name, t.Format("2006-01-02 15:04:05")})
	}
	printTable(headers, rows)
}

func bucketCreate(name string) {
	resp, err := s3Request("PUT", "/"+name, nil)
	if err != nil {
		fatal(err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 || resp.StatusCode == 204 {
		fmt.Printf("Bucket '%s' created.\n", name)
	} else if resp.StatusCode == 409 {
		fmt.Printf("Bucket '%s' already exists.\n", name)
	} else {
		body, _ := io.ReadAll(resp.Body)
		fatal(fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body)))
	}
}

func bucketDelete(name string) {
	resp, err := s3Request("DELETE", "/"+name, nil)
	if err != nil {
		fatal(err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode == 204 || resp.StatusCode == 200 {
		fmt.Printf("Bucket '%s' deleted.\n", name)
	} else {
		body, _ := io.ReadAll(resp.Body)
		fatal(fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body)))
	}
}

func bucketInfo(name string) {
	resp, err := apiRequest("GET", "/buckets/"+name, nil)
	if err != nil {
		fatal(err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		fatal(fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body)))
	}

	var info map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		fatal("parse response: " + err.Error())
	}

	data, _ := json.MarshalIndent(info, "", "  ")
	fmt.Println(string(data))
}
