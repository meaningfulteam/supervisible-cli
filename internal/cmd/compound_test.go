package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// mockCompoundServer creates a test server that handles all endpoints
// needed by compound commands.
func mockCompoundServer(t *testing.T) *httptest.Server {
	t.Helper()

	usersJSON := `{"data":[
		{"id":"u1","name":"Juan Mendez","email":"juan@m8l.com","userType":"member","isActive":true,"defaultAvailability":40,"createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:00Z"},
		{"id":"u2","name":"Maria Marquez","email":"maria@m8l.com","userType":"member","isActive":true,"defaultAvailability":40,"createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:00Z"},
		{"id":"u3","name":"Arianna","email":"arianna@m8l.com","userType":"member","isActive":false,"defaultAvailability":40,"createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:00Z"}
	]}`

	assignmentsJSON := `{"data":[
		{"id":"a1","userId":"u1","projectId":"p1","date":"2026-05-19","hours":20,"createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:00Z","project":{"id":"p1","name":"Aplazo","status":"active"}},
		{"id":"a2","userId":"u1","projectId":"p2","date":"2026-05-20","hours":12,"createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:00Z","project":{"id":"p2","name":"Zetta","status":"active"}},
		{"id":"a3","userId":"u2","projectId":"p3","date":"2026-05-19","hours":40,"createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:00Z","project":{"id":"p3","name":"SevenRooms","status":"active"}}
	]}`

	timeOffJSON := `{"data":[
		{"id":"to1","userId":"u2","timeOffTypeId":"tt1","startDate":"2026-06-01","endDate":"2026-06-05","availability":0,"status":"approved","reason":"vacation","createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:00Z","timeOffType":{"id":"tt1","name":"Vacation"}}
	]}`

	clientsJSON := `{"data":[
		{"id":"c1","organizationId":"org1","companyName":"Acme Corp","clientPriority":"high","isActive":true,"createdBy":"u1","createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:00Z","categories":[]}
	]}`

	projectsJSON := `{"data":[
		{"id":"p1","clientId":"c1","name":"Aplazo","startDate":"2026-01-01","endDate":"2026-12-31","status":"active","createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:00Z"},
		{"id":"p2","clientId":"c1","name":"Zetta","startDate":"2026-01-01","endDate":"2026-12-31","status":"active","createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:00Z"}
	]}`

	meJSON := `{"data":{"keyId":"k1","keyName":"test","organizationId":"org-123","actorUserId":"u1","scopes":["read","write"]}}`

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		path := r.URL.Path
		switch {
		case strings.HasSuffix(path, "/users"):
			_, _ = w.Write([]byte(usersJSON))
		case strings.HasSuffix(path, "/assignments"):
			_, _ = w.Write([]byte(assignmentsJSON))
		case strings.HasSuffix(path, "/time-off"):
			_, _ = w.Write([]byte(timeOffJSON))
		case strings.HasSuffix(path, "/clients"):
			_, _ = w.Write([]byte(clientsJSON))
		case strings.HasSuffix(path, "/projects"):
			_, _ = w.Write([]byte(projectsJSON))
		case strings.HasSuffix(path, "/me"):
			_, _ = w.Write([]byte(meJSON))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":{"code":"not_found","message":"not found"}}`))
		}
	}))
}

func TestCompound_Capacity_JSON(t *testing.T) {
	server := mockCompoundServer(t)
	defer server.Close()

	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", server.URL)

	stdout, _, err := executeCLI(t,
		"--config", testConfigPath(t),
		"--json",
		"capacity", "--week", "2026-W21",
	)
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}

	var report CapacityReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, stdout)
	}

	if report.Week != "2026-W21" {
		t.Errorf("expected week 2026-W21, got %s", report.Week)
	}
	if report.StartDate != "2026-05-18" {
		t.Errorf("expected start 2026-05-18, got %s", report.StartDate)
	}

	// u3 (Arianna) is inactive, should be filtered out
	if len(report.Users) != 2 {
		t.Fatalf("expected 2 active users, got %d", len(report.Users))
	}

	// Juan: 32h assigned (20+12), 40 available, 8 free
	juan := findUser(report.Users, "Juan Mendez")
	if juan == nil {
		t.Fatal("Juan not found")
	}
	if juan.AssignedHours != 32 {
		t.Errorf("Juan assigned: expected 32, got %d", juan.AssignedHours)
	}
	if juan.FreeHours != 8 {
		t.Errorf("Juan free: expected 8, got %d", juan.FreeHours)
	}
	if len(juan.Projects) != 2 {
		t.Errorf("Juan projects: expected 2, got %d", len(juan.Projects))
	}

	// Maria: 40h assigned, 40 available, 0 free
	maria := findUser(report.Users, "Maria Marquez")
	if maria == nil {
		t.Fatal("Maria not found")
	}
	if maria.AssignedHours != 40 {
		t.Errorf("Maria assigned: expected 40, got %d", maria.AssignedHours)
	}
	if maria.FreeHours != 0 {
		t.Errorf("Maria free: expected 0, got %d", maria.FreeHours)
	}
}

func TestCompound_Capacity_Table(t *testing.T) {
	server := mockCompoundServer(t)
	defer server.Close()

	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", server.URL)

	stdout, _, err := executeCLI(t,
		"--config", testConfigPath(t),
		"capacity", "--week", "2026-W21",
	)
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}

	if !strings.Contains(stdout, "Week 21") {
		t.Errorf("expected header with Week 21, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "Juan Mendez") {
		t.Errorf("expected Juan in table, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "Aplazo") {
		t.Errorf("expected Aplazo in projects column, got:\n%s", stdout)
	}
}

func TestCompound_Bench_FilterAndSort(t *testing.T) {
	server := mockCompoundServer(t)
	defer server.Close()

	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", server.URL)

	stdout, _, err := executeCLI(t,
		"--config", testConfigPath(t),
		"--json",
		"bench", "--week", "2026-W21", "--min-hours", "1",
	)
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}

	var report CapacityReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// Juan has 8h free (passes min-hours=1), Maria has 0 (filtered out)
	if len(report.Users) != 1 {
		t.Fatalf("expected 1 user with >= 1h free, got %d", len(report.Users))
	}
	if report.Users[0].Name != "Juan Mendez" {
		t.Errorf("expected Juan, got %s", report.Users[0].Name)
	}
}

func TestCompound_Bench_NoResults(t *testing.T) {
	server := mockCompoundServer(t)
	defer server.Close()

	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", server.URL)

	stdout, _, err := executeCLI(t,
		"--config", testConfigPath(t),
		"--json",
		"bench", "--week", "2026-W21", "--min-hours", "100",
	)
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}

	var report CapacityReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if len(report.Users) != 0 {
		t.Errorf("expected 0 users, got %d", len(report.Users))
	}
}

func TestCompound_Whois_ByEmail(t *testing.T) {
	server := mockCompoundServer(t)
	defer server.Close()

	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", server.URL)

	stdout, _, err := executeCLI(t,
		"--config", testConfigPath(t),
		"--json",
		"whois", "juan@m8l.com",
	)
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}

	var report WhoisReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, stdout)
	}

	if report.User.Name != "Juan Mendez" {
		t.Errorf("expected Juan Mendez, got %s", report.User.Name)
	}
	if report.User.Email != "juan@m8l.com" {
		t.Errorf("expected juan@m8l.com, got %s", report.User.Email)
	}
	// Mock returns all assignments (doesn't filter by user_id), so we get all 3
	if len(report.Assignments) != 3 {
		t.Errorf("expected 3 assignments (mock doesn't filter), got %d", len(report.Assignments))
	}
}

func TestCompound_Whois_ByName(t *testing.T) {
	server := mockCompoundServer(t)
	defer server.Close()

	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", server.URL)

	stdout, _, err := executeCLI(t,
		"--config", testConfigPath(t),
		"--json",
		"whois", "juan",
	)
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}

	var report WhoisReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if report.User.Name != "Juan Mendez" {
		t.Errorf("expected Juan Mendez, got %s", report.User.Name)
	}
}

func TestCompound_Whois_NoMatch(t *testing.T) {
	server := mockCompoundServer(t)
	defer server.Close()

	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", server.URL)

	_, _, err := executeCLI(t,
		"--config", testConfigPath(t),
		"--json",
		"whois", "nonexistent",
	)
	if err == nil {
		t.Fatal("expected error for no match")
	}
	if !strings.Contains(err.Error(), "no user found") {
		t.Errorf("expected 'no user found' error, got: %v", err)
	}
}

func TestCompound_Whois_MultipleMatches(t *testing.T) {
	server := mockCompoundServer(t)
	defer server.Close()

	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", server.URL)

	// "ma" matches both "Juan Mendez" (no) and "Maria Marquez" (yes) and "Arianna" (no)
	// Actually "ma" matches "Maria Marquez" only in name. Let me use "a" which matches
	// Juan Mendez, Maria Marquez, and Arianna
	_, _, err := executeCLI(t,
		"--config", testConfigPath(t),
		"--json",
		"whois", "a",
	)
	if err == nil {
		t.Fatal("expected error for multiple matches")
	}
	if !strings.Contains(err.Error(), "multiple users match") {
		t.Errorf("expected 'multiple users match' error, got: %v", err)
	}
}

func TestCompound_Context_JSON(t *testing.T) {
	server := mockCompoundServer(t)
	defer server.Close()

	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", server.URL)

	stdout, _, err := executeCLI(t,
		"--config", testConfigPath(t),
		"--json",
		"context",
	)
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}

	var report ContextReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, stdout)
	}

	if report.Organization != "org-123" {
		t.Errorf("expected org-123, got %s", report.Organization)
	}
	if len(report.Users) != 3 {
		t.Errorf("expected 3 users (all, not filtered), got %d", len(report.Users))
	}
	if len(report.Clients) != 1 {
		t.Errorf("expected 1 client, got %d", len(report.Clients))
	}
	if len(report.Projects) != 2 {
		t.Errorf("expected 2 projects, got %d", len(report.Projects))
	}
}

func TestCompound_Context_Table(t *testing.T) {
	server := mockCompoundServer(t)
	defer server.Close()

	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", server.URL)

	stdout, _, err := executeCLI(t,
		"--config", testConfigPath(t),
		"context",
	)
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}

	if !strings.Contains(stdout, "3 users") {
		t.Errorf("expected '3 users' in summary, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "1 clients") {
		t.Errorf("expected '1 clients' in summary, got:\n%s", stdout)
	}
}

func findUser(users []UserCapacity, name string) *UserCapacity {
	for i := range users {
		if users[i].Name == name {
			return &users[i]
		}
	}
	return nil
}
