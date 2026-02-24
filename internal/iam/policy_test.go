package iam

import "testing"

func TestEvaluate_AllowAll(t *testing.T) {
	policies := []Policy{{
		Version: "2012-10-17",
		Statement: []Statement{{
			Effect:   "Allow",
			Action:   []string{"s3:*"},
			Resource: []string{"*"},
		}},
	}}
	if !Evaluate(policies, "s3:GetObject", "arn:aws:s3:::mybucket/file.txt") {
		t.Error("expected Allow for s3:* / *")
	}
}

func TestEvaluate_DefaultDeny(t *testing.T) {
	policies := []Policy{{
		Version:   "2012-10-17",
		Statement: []Statement{},
	}}
	if Evaluate(policies, "s3:GetObject", "arn:aws:s3:::mybucket/file.txt") {
		t.Error("expected default Deny with no statements")
	}
}

func TestEvaluate_ExplicitDenyWins(t *testing.T) {
	policies := []Policy{{
		Version: "2012-10-17",
		Statement: []Statement{
			{Effect: "Allow", Action: []string{"s3:*"}, Resource: []string{"*"}},
			{Effect: "Deny", Action: []string{"s3:DeleteObject"}, Resource: []string{"*"}},
		},
	}}
	if Evaluate(policies, "s3:DeleteObject", "arn:aws:s3:::mybucket/file.txt") {
		t.Error("expected Deny to override Allow")
	}
	if !Evaluate(policies, "s3:GetObject", "arn:aws:s3:::mybucket/file.txt") {
		t.Error("expected Allow for non-denied action")
	}
}

func TestEvaluate_ResourcePrefix(t *testing.T) {
	policies := []Policy{{
		Version: "2012-10-17",
		Statement: []Statement{{
			Effect:   "Allow",
			Action:   []string{"s3:GetObject"},
			Resource: []string{"arn:aws:s3:::mybucket/*"},
		}},
	}}
	if !Evaluate(policies, "s3:GetObject", "arn:aws:s3:::mybucket/dir/file.txt") {
		t.Error("expected Allow for matching resource prefix")
	}
	if Evaluate(policies, "s3:GetObject", "arn:aws:s3:::otherbucket/file.txt") {
		t.Error("expected Deny for non-matching resource")
	}
}

func TestEvaluate_ActionWildcard(t *testing.T) {
	policies := []Policy{{
		Version: "2012-10-17",
		Statement: []Statement{{
			Effect:   "Allow",
			Action:   []string{"s3:Get*"},
			Resource: []string{"*"},
		}},
	}}
	if !Evaluate(policies, "s3:GetObject", "*") {
		t.Error("expected Allow for s3:Get*")
	}
	if Evaluate(policies, "s3:PutObject", "*") {
		t.Error("expected Deny for s3:PutObject against s3:Get*")
	}
}

func TestMatchWildcard(t *testing.T) {
	tests := []struct {
		pattern, value string
		want           bool
	}{
		{"*", "anything", true},
		{"s3:*", "s3:GetObject", true},
		{"s3:Get*", "s3:GetObject", true},
		{"s3:Get*", "s3:PutObject", false},
		{"arn:aws:s3:::bucket/*", "arn:aws:s3:::bucket/key", true},
		{"arn:aws:s3:::bucket/*", "arn:aws:s3:::bucket", true},
		{"arn:aws:s3:::bucket", "arn:aws:s3:::bucket", true},
		{"arn:aws:s3:::bucket", "arn:aws:s3:::other", false},
	}
	for _, tt := range tests {
		got := matchWildcard(tt.pattern, tt.value)
		if got != tt.want {
			t.Errorf("matchWildcard(%q, %q) = %v, want %v", tt.pattern, tt.value, got, tt.want)
		}
	}
}

func TestEvaluate_MultiplePolicies(t *testing.T) {
	policies := []Policy{
		{Statement: []Statement{{Effect: "Allow", Action: []string{"s3:GetObject"}, Resource: []string{"*"}}}},
		{Statement: []Statement{{Effect: "Allow", Action: []string{"s3:PutObject"}, Resource: []string{"*"}}}},
	}
	if !Evaluate(policies, "s3:GetObject", "*") {
		t.Error("expected Allow from first policy")
	}
	if !Evaluate(policies, "s3:PutObject", "*") {
		t.Error("expected Allow from second policy")
	}
	if Evaluate(policies, "s3:DeleteObject", "*") {
		t.Error("expected Deny for action not in either policy")
	}
}
