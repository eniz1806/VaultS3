package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

func runUser(args []string) {
	if len(args) == 0 {
		fmt.Println(`Usage: vaults3-cli user <subcommand>

Subcommands:
  list                                   List IAM users
  create <name> --access-key=<ak> --secret-key=<sk>   Create IAM user
  delete <name>                          Delete IAM user
  attach-policy <user> <policy>          Attach policy to user`)
		os.Exit(1)
	}

	requireCreds()

	switch args[0] {
	case "list", "ls":
		userList()
	case "create":
		userCreate(args[1:])
	case "delete", "rm":
		if len(args) < 2 {
			fatal("user delete requires a username")
		}
		userDelete(args[1])
	case "attach-policy":
		if len(args) < 3 {
			fatal("user attach-policy requires: <user> <policy>")
		}
		userAttachPolicy(args[1], args[2])
	default:
		fatal("unknown user subcommand: " + args[0])
	}
}

func userList() {
	resp, err := apiRequest("GET", "/iam/users", nil)
	if err != nil {
		fatal(err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		fatal(fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body)))
	}

	var users []struct {
		UserID   string   `json:"user_id"`
		UserName string   `json:"user_name"`
		Policies []string `json:"policies"`
		Groups   []string `json:"groups"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		fatal("parse response: " + err.Error())
	}

	if len(users) == 0 {
		fmt.Println("No IAM users found.")
		return
	}

	headers := []string{"USER ID", "USER NAME", "POLICIES", "GROUPS"}
	var rows [][]string
	for _, u := range users {
		rows = append(rows, []string{
			u.UserID,
			u.UserName,
			strings.Join(u.Policies, ", "),
			strings.Join(u.Groups, ", "),
		})
	}
	printTable(headers, rows)
}

func userCreate(args []string) {
	if len(args) < 1 {
		fatal("user create requires a username")
	}
	name := args[0]
	ak := ""
	sk := ""

	for _, arg := range args[1:] {
		if strings.HasPrefix(arg, "--access-key=") {
			ak = strings.TrimPrefix(arg, "--access-key=")
		} else if strings.HasPrefix(arg, "--secret-key=") {
			sk = strings.TrimPrefix(arg, "--secret-key=")
		}
	}

	payload := map[string]string{
		"user_name": name,
	}
	if ak != "" {
		payload["access_key"] = ak
	}
	if sk != "" {
		payload["secret_key"] = sk
	}

	data, _ := json.Marshal(payload)
	resp, err := apiRequest("POST", "/iam/users", bytes.NewReader(data))
	if err != nil {
		fatal(err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 || resp.StatusCode == 201 {
		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)
		out, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(out))
	} else {
		body, _ := io.ReadAll(resp.Body)
		fatal(fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body)))
	}
}

func userDelete(name string) {
	resp, err := apiRequest("DELETE", "/iam/users/"+name, nil)
	if err != nil {
		fatal(err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 || resp.StatusCode == 204 {
		fmt.Printf("User '%s' deleted.\n", name)
	} else {
		body, _ := io.ReadAll(resp.Body)
		fatal(fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body)))
	}
}

func userAttachPolicy(userName, policyName string) {
	payload := map[string]string{"policy_name": policyName}
	data, _ := json.Marshal(payload)
	resp, err := apiRequest("POST", "/iam/users/"+userName+"/policies", bytes.NewReader(data))
	if err != nil {
		fatal(err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 || resp.StatusCode == 204 {
		fmt.Printf("Policy '%s' attached to user '%s'.\n", policyName, userName)
	} else {
		body, _ := io.ReadAll(resp.Body)
		fatal(fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body)))
	}
}
