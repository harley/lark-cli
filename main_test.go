package main

import (
	"net/url"
	"testing"
)

func TestBuildUsersListPath(t *testing.T) {
	path := buildUsersListPath("open_department_id", "0", "open_id", "name,email", 50, "abc+/=")

	u, err := url.Parse(path)
	if err != nil {
		t.Fatalf("parse path: %v", err)
	}
	query := u.Query()

	if got := query.Get("department_id_type"); got != "open_department_id" {
		t.Fatalf("department_id_type = %q", got)
	}
	if got := query.Get("department_id"); got != "0" {
		t.Fatalf("department_id = %q", got)
	}
	if got := query.Get("user_id_type"); got != "open_id" {
		t.Fatalf("user_id_type = %q", got)
	}
	if got := query.Get("fields"); got != "name,email" {
		t.Fatalf("fields = %q", got)
	}
	if got := query.Get("page_size"); got != "50" {
		t.Fatalf("page_size = %q", got)
	}
	if got := query.Get("page_token"); got != "abc+/=" {
		t.Fatalf("page_token = %q", got)
	}
}

func TestUniqueUsers(t *testing.T) {
	input := []userProfile{
		{OpenID: "ou_1", Name: "Alpha"},
		{OpenID: "ou_1", Name: "Alpha Dup"},
		{UserID: "u_1", Name: "UserIDUser"},
		{UserID: "u_1", Name: "UserIDUser Dup"},
		{Email: "user@example.com", Name: "EmailUser"},
		{Email: "USER@example.com", Name: "EmailUser Dup"},
		{Name: "Fallback", Mobile: "123"},
		{Name: "Fallback", Mobile: "123"},
	}

	output := uniqueUsers(input)
	if len(output) != 5 {
		t.Fatalf("expected 5 unique users, got %d", len(output))
	}
}

func TestSortUsers(t *testing.T) {
	input := []userProfile{
		{Name: "zoe", Email: "z@example.com", OpenID: "ou_3"},
		{Name: "Anna", Email: "b@example.com", OpenID: "ou_2"},
		{Name: "anna", Email: "a@example.com", OpenID: "ou_1"},
	}

	sortUsers(input)

	if input[0].Email != "a@example.com" || input[1].Email != "b@example.com" || input[2].Email != "z@example.com" {
		t.Fatalf("unexpected sort order: %#v", input)
	}
}

func TestRedactToken(t *testing.T) {
	if got := redactToken("token123456789"); got != "token123...redacted" {
		t.Fatalf("unexpected redaction: %q", got)
	}
	if got := redactToken("abcd"); got != "***redacted" {
		t.Fatalf("unexpected short-token redaction: %q", got)
	}
}

func TestAppendQueryParams(t *testing.T) {
	path, err := appendQueryParams("/open-apis/im/v1/chats?existing=1", map[string]interface{}{
		"page_size": float64(5),
		"has_more":  true,
		"labels":    []interface{}{"a", "b"},
	})
	if err != nil {
		t.Fatalf("append query params: %v", err)
	}

	u, err := url.Parse(path)
	if err != nil {
		t.Fatalf("parse path: %v", err)
	}
	query := u.Query()

	if got := query.Get("existing"); got != "1" {
		t.Fatalf("existing = %q", got)
	}
	if got := query.Get("page_size"); got != "5" {
		t.Fatalf("page_size = %q", got)
	}
	if got := query.Get("has_more"); got != "true" {
		t.Fatalf("has_more = %q", got)
	}
	labels := query["labels"]
	if len(labels) != 2 || labels[0] != "a" || labels[1] != "b" {
		t.Fatalf("labels = %#v", labels)
	}
}

func TestAppendQueryParamsInvalidType(t *testing.T) {
	_, err := appendQueryParams("/open-apis/im/v1/chats", []interface{}{"bad"})
	if err == nil {
		t.Fatal("expected error for non-object params")
	}
}
