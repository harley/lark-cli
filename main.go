package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/go-lark/lark"
	"github.com/joho/godotenv"
)

const (
	appVersion = "0.1.0"
)

type rootConfig struct {
	AppID      string
	AppSecret  string
	Domain     string
	OutputMode string
	UserIDType string
}

type envelope struct {
	OK      bool        `json:"ok"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

type usersFindByDepartmentResponse struct {
	Code int                   `json:"code"`
	Msg  string                `json:"msg"`
	Data usersFindByDepartment `json:"data"`
}

type usersFindByDepartment struct {
	HasMore   bool          `json:"has_more"`
	PageToken string        `json:"page_token"`
	Items     []userProfile `json:"items"`
}

type userProfile struct {
	Name            string   `json:"name,omitempty"`
	Email           string   `json:"email,omitempty"`
	EnterpriseEmail string   `json:"enterprise_email,omitempty"`
	Mobile          string   `json:"mobile,omitempty"`
	MobileVisible   bool     `json:"mobile_visible,omitempty"`
	OpenID          string   `json:"open_id,omitempty"`
	UnionID         string   `json:"union_id,omitempty"`
	JobTitle        string   `json:"job_title,omitempty"`
	DepartmentIDs   []string `json:"department_ids,omitempty"`
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, os.Stdin))
}

func run(args []string, stdout io.Writer, stderr io.Writer, stdin io.Reader) int {
	// Best-effort .env loading for local/dev agent workflows.
	_ = godotenv.Load()

	cfg, remaining, err := parseRootFlags(args)
	if err != nil {
		_ = writeError(stderr, "json", err)
		return 2
	}

	if len(remaining) == 0 {
		printRootUsage(stderr)
		return 2
	}

	cmd := remaining[0]
	cmdArgs := remaining[1:]

	switch cmd {
	case "help", "--help", "-h":
		printRootUsage(stdout)
		return 0
	case "version":
		if err := writeSuccess(stdout, cfg.OutputMode, map[string]string{"version": appVersion}); err != nil {
			_ = writeError(stderr, cfg.OutputMode, err)
			return 1
		}
		return 0
	case "auth":
		result, err := runAuth(cfg, cmdArgs)
		if err != nil {
			_ = writeError(stderr, cfg.OutputMode, err)
			return 1
		}
		_ = writeSuccess(stdout, cfg.OutputMode, result)
		return 0
	case "msg":
		result, err := runMsg(cfg, cmdArgs, stdin)
		if err != nil {
			_ = writeError(stderr, cfg.OutputMode, err)
			return 1
		}
		_ = writeSuccess(stdout, cfg.OutputMode, result)
		return 0
	case "api":
		result, err := runAPI(cfg, cmdArgs, stdin)
		if err != nil {
			_ = writeError(stderr, cfg.OutputMode, err)
			return 1
		}
		_ = writeSuccess(stdout, cfg.OutputMode, result)
		return 0
	case "users":
		result, err := runUsers(cfg, cmdArgs)
		if err != nil {
			_ = writeError(stderr, cfg.OutputMode, err)
			return 1
		}
		_ = writeSuccess(stdout, cfg.OutputMode, result)
		return 0
	default:
		_ = writeError(stderr, cfg.OutputMode, fmt.Errorf("unknown command %q", cmd))
		return 2
	}
}

func parseRootFlags(args []string) (rootConfig, []string, error) {
	cfg := rootConfig{
		AppID:      strings.TrimSpace(os.Getenv("LARK_APP_ID")),
		AppSecret:  strings.TrimSpace(os.Getenv("LARK_APP_SECRET")),
		Domain:     strings.TrimSpace(os.Getenv("LARK_DOMAIN")),
		OutputMode: strings.TrimSpace(os.Getenv("LARK_OUTPUT")),
		UserIDType: strings.TrimSpace(os.Getenv("LARK_USER_ID_TYPE")),
	}

	if cfg.OutputMode == "" {
		cfg.OutputMode = "json"
	}

	fs := flag.NewFlagSet("lark-cli", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&cfg.AppID, "app-id", cfg.AppID, "Lark app ID (or LARK_APP_ID)")
	fs.StringVar(&cfg.AppSecret, "app-secret", cfg.AppSecret, "Lark app secret (or LARK_APP_SECRET)")
	fs.StringVar(&cfg.Domain, "domain", cfg.Domain, "Domain: feishu, lark, or full base URL")
	fs.StringVar(&cfg.OutputMode, "output", cfg.OutputMode, "Output mode: json or text")
	fs.StringVar(&cfg.UserIDType, "user-id-type", cfg.UserIDType, "Preferred user id type")

	if err := fs.Parse(args); err != nil {
		return cfg, nil, err
	}

	cfg.OutputMode = strings.ToLower(strings.TrimSpace(cfg.OutputMode))
	if cfg.OutputMode != "json" && cfg.OutputMode != "text" {
		return cfg, nil, fmt.Errorf("invalid output mode %q (expected json or text)", cfg.OutputMode)
	}

	if _, err := normalizedDomain(cfg.Domain); err != nil {
		return cfg, nil, err
	}

	return cfg, fs.Args(), nil
}

func runAuth(cfg rootConfig, args []string) (interface{}, error) {
	if len(args) == 0 {
		return nil, errors.New("missing auth subcommand (expected: tenant-token)")
	}

	switch args[0] {
	case "tenant-token":
		fs := flag.NewFlagSet("tenant-token", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		refresh := fs.Bool("refresh", true, "force token refresh")
		if err := fs.Parse(args[1:]); err != nil {
			return nil, err
		}

		bot, err := newChatBot(cfg)
		if err != nil {
			return nil, err
		}

		resp, err := bot.GetTenantAccessTokenInternal(*refresh)
		if err != nil {
			return nil, err
		}
		if resp.Code != 0 {
			return nil, fmt.Errorf("lark auth failed: code=%d msg=%s", resp.Code, resp.Msg)
		}

		return map[string]interface{}{
			"tenant_access_token": resp.TenantAppAccessToken,
			"expire_seconds":      resp.Expire,
		}, nil
	default:
		return nil, fmt.Errorf("unknown auth subcommand %q", args[0])
	}
}

func runMsg(cfg rootConfig, args []string, stdin io.Reader) (interface{}, error) {
	if len(args) == 0 {
		return nil, errors.New("missing msg subcommand (expected: text or send)")
	}

	switch args[0] {
	case "text":
		fs := flag.NewFlagSet("msg text", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		toType := fs.String("to-type", "", "Target id type: chat_id|open_chat_id|open_id|user_id|email|union_id")
		to := fs.String("to", "", "Target id value")
		text := fs.String("text", "", "Message text")
		replyID := fs.String("reply-id", "", "Reply root message id")
		replyInThread := fs.Bool("reply-in-thread", false, "Reply in thread")
		if err := fs.Parse(args[1:]); err != nil {
			return nil, err
		}

		if *toType == "" || *to == "" {
			return nil, errors.New("msg text requires --to-type and --to")
		}
		if strings.TrimSpace(*text) == "" {
			return nil, errors.New("msg text requires --text")
		}

		bot, err := newChatBot(cfg)
		if err != nil {
			return nil, err
		}
		if err := primeTenantToken(bot); err != nil {
			return nil, err
		}

		buf := lark.NewMsgBuffer(lark.MsgText).Text(*text)
		if err := bindTarget(buf, *toType, *to); err != nil {
			return nil, err
		}
		if *replyID != "" {
			buf.BindReply(*replyID)
		}
		if *replyInThread {
			buf.ReplyInThread(true)
		}
		if err := buf.Error(); err != nil {
			return nil, err
		}

		resp, err := bot.PostMessage(buf.Build())
		if err != nil {
			return nil, err
		}
		if resp.Code != 0 {
			return nil, fmt.Errorf("lark msg failed: code=%d msg=%s", resp.Code, resp.Msg)
		}

		return map[string]interface{}{
			"message_id": resp.Data.MessageID,
			"chat_id":    resp.Data.ChatID,
			"msg_type":   resp.Data.MsgType,
		}, nil

	case "send":
		fs := flag.NewFlagSet("msg send", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		input := fs.String("input", "-", "JSON source: '-' for stdin, inline JSON, or '@path/to/file.json'")
		if err := fs.Parse(args[1:]); err != nil {
			return nil, err
		}

		raw, err := readInput(*input, stdin)
		if err != nil {
			return nil, err
		}

		var message lark.OutcomingMessage
		if err := json.Unmarshal(raw, &message); err != nil {
			return nil, fmt.Errorf("invalid message json: %w", err)
		}
		if strings.TrimSpace(message.MsgType) == "" {
			return nil, errors.New("msg send requires msg_type in payload")
		}

		bot, err := newChatBot(cfg)
		if err != nil {
			return nil, err
		}
		if err := primeTenantToken(bot); err != nil {
			return nil, err
		}

		resp, err := bot.PostMessage(message)
		if err != nil {
			return nil, err
		}
		if resp.Code != 0 {
			return nil, fmt.Errorf("lark msg failed: code=%d msg=%s", resp.Code, resp.Msg)
		}

		return map[string]interface{}{
			"message_id": resp.Data.MessageID,
			"chat_id":    resp.Data.ChatID,
			"msg_type":   resp.Data.MsgType,
		}, nil
	default:
		return nil, fmt.Errorf("unknown msg subcommand %q", args[0])
	}
}

func runAPI(cfg rootConfig, args []string, stdin io.Reader) (interface{}, error) {
	if len(args) == 0 {
		return nil, errors.New("missing api subcommand (expected: call)")
	}

	if args[0] != "call" {
		return nil, fmt.Errorf("unknown api subcommand %q", args[0])
	}

	fs := flag.NewFlagSet("api call", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	method := fs.String("method", "GET", "HTTP method: GET|POST|PUT|PATCH|DELETE")
	path := fs.String("path", "", "Open API path (e.g. /open-apis/im/v1/chats)")
	auth := fs.Bool("auth", true, "Use auth token")
	paramsInput := fs.String("params", "", "JSON source: inline JSON, '-', or '@path/to/file.json'")
	if err := fs.Parse(args[1:]); err != nil {
		return nil, err
	}

	if strings.TrimSpace(*path) == "" {
		return nil, errors.New("api call requires --path")
	}

	bot, err := newChatBot(cfg)
	if err != nil {
		return nil, err
	}
	if *auth {
		if err := primeTenantToken(bot); err != nil {
			return nil, err
		}
	}

	var params interface{}
	if strings.TrimSpace(*paramsInput) != "" {
		raw, err := readInput(*paramsInput, stdin)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(raw, &params); err != nil {
			return nil, fmt.Errorf("invalid params json: %w", err)
		}
	}

	methodValue := strings.ToUpper(strings.TrimSpace(*method))
	out := map[string]interface{}{}
	switch methodValue {
	case "GET":
		err = bot.GetAPIRequest("lark-cli", *path, *auth, params, &out)
	case "POST":
		err = bot.PostAPIRequest("lark-cli", *path, *auth, params, &out)
	case "PUT":
		err = bot.PutAPIRequest("lark-cli", *path, *auth, params, &out)
	case "PATCH":
		err = bot.PatchAPIRequest("lark-cli", *path, *auth, params, &out)
	case "DELETE":
		err = bot.DeleteAPIRequest("lark-cli", *path, *auth, params, &out)
	default:
		return nil, fmt.Errorf("unsupported method %q", methodValue)
	}
	if err != nil {
		return nil, err
	}

	if code, ok := out["code"].(float64); ok && int(code) != 0 {
		msg := ""
		if rawMsg, ok := out["msg"].(string); ok {
			msg = rawMsg
		}
		return nil, fmt.Errorf("lark api failed: code=%d msg=%s", int(code), msg)
	}

	return out, nil
}

func runUsers(cfg rootConfig, args []string) (interface{}, error) {
	if len(args) == 0 {
		return nil, errors.New("missing users subcommand (expected: list)")
	}
	if args[0] != "list" {
		return nil, fmt.Errorf("unknown users subcommand %q", args[0])
	}

	fs := flag.NewFlagSet("users list", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	departmentID := fs.String("department-id", "0", "Department ID")
	departmentIDType := fs.String("department-id-type", "open_department_id", "Department ID type")
	userIDType := fs.String("user-id-type", "", "User ID type for API query (default: open_id or global --user-id-type)")
	fields := fs.String("fields", "name,email,enterprise_email,mobile,department_ids,job_title", "Comma-separated user fields")
	pageSize := fs.Int("page-size", 50, "Page size (1-50)")
	maxPages := fs.Int("max-pages", 0, "Max pages to fetch (0 = all)")
	retries := fs.Int("retries", 4, "Retries per page on transient errors")
	retryDelayMS := fs.Int("retry-delay-ms", 1000, "Retry delay in milliseconds")
	if err := fs.Parse(args[1:]); err != nil {
		return nil, err
	}

	if strings.TrimSpace(*departmentID) == "" {
		return nil, errors.New("users list requires --department-id")
	}
	if *pageSize < 1 || *pageSize > 50 {
		return nil, errors.New("--page-size must be between 1 and 50")
	}
	if *maxPages < 0 {
		return nil, errors.New("--max-pages must be >= 0")
	}
	if *retries < 1 {
		return nil, errors.New("--retries must be >= 1")
	}
	if *retryDelayMS < 0 {
		return nil, errors.New("--retry-delay-ms must be >= 0")
	}

	resolvedUserIDType := strings.TrimSpace(*userIDType)
	if resolvedUserIDType == "" {
		resolvedUserIDType = strings.TrimSpace(cfg.UserIDType)
	}
	if resolvedUserIDType == "" {
		resolvedUserIDType = "open_id"
	}

	bot, err := newChatBot(cfg)
	if err != nil {
		return nil, err
	}
	if err := primeTenantToken(bot); err != nil {
		return nil, err
	}

	var (
		pageToken string
		pages     int
		allUsers  []userProfile
	)
	for {
		path := buildUsersListPath(*departmentIDType, *departmentID, resolvedUserIDType, strings.TrimSpace(*fields), *pageSize, pageToken)
		pageResp, err := fetchUsersPageWithRetry(bot, path, *retries, time.Duration(*retryDelayMS)*time.Millisecond)
		if err != nil {
			return nil, err
		}

		pages++
		allUsers = append(allUsers, pageResp.Data.Items...)

		if !pageResp.Data.HasMore || strings.TrimSpace(pageResp.Data.PageToken) == "" {
			break
		}
		if *maxPages > 0 && pages >= *maxPages {
			break
		}

		pageToken = pageResp.Data.PageToken
	}

	users := uniqueUsers(allUsers)
	sortUsers(users)

	return map[string]interface{}{
		"department_id":      *departmentID,
		"department_id_type": *departmentIDType,
		"user_id_type":       resolvedUserIDType,
		"pages":              pages,
		"count":              len(users),
		"users":              users,
	}, nil
}

func newChatBot(cfg rootConfig) (*lark.Bot, error) {
	if strings.TrimSpace(cfg.AppID) == "" {
		return nil, errors.New("missing app id: use --app-id or LARK_APP_ID")
	}
	if strings.TrimSpace(cfg.AppSecret) == "" {
		return nil, errors.New("missing app secret: use --app-secret or LARK_APP_SECRET")
	}

	bot := lark.NewChatBot(cfg.AppID, cfg.AppSecret)
	domain, err := normalizedDomain(cfg.Domain)
	if err != nil {
		return nil, err
	}
	if domain != "" {
		bot.SetDomain(domain)
	}
	if strings.TrimSpace(cfg.UserIDType) != "" {
		bot.WithUserIDType(cfg.UserIDType)
	}

	return bot, nil
}

func primeTenantToken(bot *lark.Bot) error {
	resp, err := bot.GetTenantAccessTokenInternal(true)
	if err != nil {
		return err
	}
	if resp.Code != 0 {
		return fmt.Errorf("lark auth failed: code=%d msg=%s", resp.Code, resp.Msg)
	}
	return nil
}

func fetchUsersPageWithRetry(bot *lark.Bot, path string, retries int, delay time.Duration) (usersFindByDepartmentResponse, error) {
	var resp usersFindByDepartmentResponse

	for attempt := 1; attempt <= retries; attempt++ {
		err := bot.GetAPIRequest("UsersList", path, true, nil, &resp)
		if err == nil {
			if resp.Code == 0 {
				return resp, nil
			}
			if resp.Code == 2200 && attempt < retries {
				if delay > 0 {
					time.Sleep(delay)
				}
				continue
			}
			return resp, fmt.Errorf("lark users list failed: code=%d msg=%s", resp.Code, resp.Msg)
		}

		if attempt == retries {
			return resp, err
		}
		if delay > 0 {
			time.Sleep(delay)
		}
	}

	return resp, errors.New("failed to fetch users page")
}

func buildUsersListPath(departmentIDType, departmentID, userIDType, fields string, pageSize int, pageToken string) string {
	values := url.Values{}
	values.Set("department_id_type", departmentIDType)
	values.Set("department_id", departmentID)
	values.Set("user_id_type", userIDType)
	values.Set("page_size", fmt.Sprintf("%d", pageSize))
	if strings.TrimSpace(fields) != "" {
		values.Set("fields", fields)
	}
	if strings.TrimSpace(pageToken) != "" {
		values.Set("page_token", pageToken)
	}

	return "/open-apis/contact/v3/users/find_by_department?" + values.Encode()
}

func uniqueUsers(items []userProfile) []userProfile {
	seen := make(map[string]struct{}, len(items))
	unique := make([]userProfile, 0, len(items))

	for _, item := range items {
		key := strings.TrimSpace(item.OpenID)
		if key == "" {
			key = strings.TrimSpace(item.UnionID)
		}
		if key == "" {
			key = strings.ToLower(strings.TrimSpace(item.Email))
		}
		if key == "" {
			key = strings.ToLower(strings.TrimSpace(item.Name)) + "|" + strings.TrimSpace(item.Mobile)
		}
		if key == "" {
			key = fmt.Sprintf("anonymous-%d", len(unique))
		}

		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, item)
	}

	return unique
}

func sortUsers(users []userProfile) {
	sort.Slice(users, func(i, j int) bool {
		nameI := strings.ToLower(strings.TrimSpace(users[i].Name))
		nameJ := strings.ToLower(strings.TrimSpace(users[j].Name))
		if nameI != nameJ {
			return nameI < nameJ
		}

		emailI := strings.ToLower(strings.TrimSpace(users[i].Email))
		emailJ := strings.ToLower(strings.TrimSpace(users[j].Email))
		if emailI != emailJ {
			return emailI < emailJ
		}

		openIDI := strings.TrimSpace(users[i].OpenID)
		openIDJ := strings.TrimSpace(users[j].OpenID)
		return openIDI < openIDJ
	})
}

func normalizedDomain(value string) (string, error) {
	domain := strings.TrimSpace(value)
	if domain == "" {
		return "", nil
	}

	switch strings.ToLower(domain) {
	case "lark":
		return lark.DomainLark, nil
	case "feishu":
		return lark.DomainFeishu, nil
	}

	if strings.HasPrefix(domain, "http://") || strings.HasPrefix(domain, "https://") {
		return domain, nil
	}

	return "", fmt.Errorf("invalid domain %q (expected lark, feishu, or full URL)", domain)
}

func bindTarget(buf *lark.MsgBuffer, toType string, to string) error {
	switch strings.ToLower(strings.TrimSpace(toType)) {
	case "chat_id":
		buf.BindChatID(to)
	case "open_chat_id":
		buf.BindOpenChatID(to)
	case "open_id":
		buf.BindOpenID(to)
	case "user_id":
		buf.BindUserID(to)
	case "email":
		buf.BindEmail(to)
	case "union_id":
		buf.BindUnionID(to)
	default:
		return fmt.Errorf("unsupported --to-type %q", toType)
	}
	return nil
}

func readInput(input string, stdin io.Reader) ([]byte, error) {
	source := strings.TrimSpace(input)
	if source == "" {
		return nil, errors.New("empty input")
	}

	switch {
	case source == "-":
		return io.ReadAll(stdin)
	case strings.HasPrefix(source, "@"):
		path := strings.TrimSpace(strings.TrimPrefix(source, "@"))
		if path == "" {
			return nil, errors.New("missing file path after '@'")
		}
		return os.ReadFile(path)
	default:
		return []byte(source), nil
	}
}

func writeSuccess(w io.Writer, mode string, data interface{}) error {
	switch mode {
	case "json":
		return json.NewEncoder(w).Encode(envelope{OK: true, Data: data})
	case "text":
		if data == nil {
			_, err := fmt.Fprintln(w, "ok")
			return err
		}
		asJSON, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(w, string(asJSON))
		return err
	default:
		return fmt.Errorf("unsupported output mode %q", mode)
	}
}

func writeError(w io.Writer, mode string, err error) error {
	switch mode {
	case "json":
		return json.NewEncoder(w).Encode(envelope{OK: false, Error: err.Error()})
	case "text":
		_, writeErr := fmt.Fprintln(w, "error:", err.Error())
		return writeErr
	default:
		_, writeErr := fmt.Fprintln(w, err.Error())
		return writeErr
	}
}

func printRootUsage(w io.Writer) {
	_, _ = fmt.Fprintln(w, `lark-cli - agent-friendly CLI for Lark Open Platform

Usage:
  lark-cli [global-flags] <command> <subcommand> [flags]

Global flags:
  --app-id         Lark app ID (env: LARK_APP_ID)
  --app-secret     Lark app secret (env: LARK_APP_SECRET)
  --domain         lark | feishu | https://custom.domain
  --output         json | text (default: json)
  --user-id-type   custom user id type

Commands:
  auth tenant-token
  msg text
  msg send
  api call
  users list
  version

Examples:
  lark-cli auth tenant-token
  lark-cli msg text --to-type chat_id --to oc_xxx --text "hello"
  lark-cli msg send --input @message.json
  lark-cli api call --method GET --path /open-apis/im/v1/chats --params '{"page_size": 20}'
  lark-cli users list --department-id 0 --fields name,email,department_ids`)
}
