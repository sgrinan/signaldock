package web

import (
	"context"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"uuid"

	"github.com/sgrinan/signaldock/internal/endpoint"
	"github.com/sgrinan/signaldock/internal/session"
	"github.com/sgrinan/signaldock/internal/user"
)

type fakeEndpointService struct {
	addFunc        func(string) (endpoint.Endpoint, error)
	listFunc       func() ([]endpoint.Endpoint, error)
	byIDFunc       func(uuid.UUID) (endpoint.Endpoint, error)
	removeByIDFunc func(uuid.UUID) error
	refreshFunc    func(uuid.UUID) (endpoint.CheckResult, error)
}

var _ endpointService = (*fakeEndpointService)(nil)

func (f *fakeEndpointService) Add(rawURL string) (endpoint.Endpoint, error) {
	if f.addFunc != nil {
		return f.addFunc(rawURL)
	}

	return endpoint.Endpoint{}, nil
}

func (f *fakeEndpointService) List() ([]endpoint.Endpoint, error) {
	if f.listFunc != nil {
		return f.listFunc()
	}

	return nil, nil
}

func (f *fakeEndpointService) ByID(id uuid.UUID) (endpoint.Endpoint, error) {
	if f.byIDFunc != nil {
		return f.byIDFunc(id)
	}

	return endpoint.Endpoint{}, endpoint.ErrEndpointNotFound
}

func (f *fakeEndpointService) RemoveByID(id uuid.UUID) error {
	if f.removeByIDFunc != nil {
		return f.removeByIDFunc(id)
	}

	return nil
}

func (f *fakeEndpointService) Refresh(id uuid.UUID) (endpoint.CheckResult, error) {
	if f.refreshFunc != nil {
		return f.refreshFunc(id)
	}

	return endpoint.CheckResult{}, nil
}

type fakeUserRepository struct {
	insertFunc      func(user.User) error
	listFunc        func() ([]user.User, error)
	byUsernameFunc  func(string) (user.User, error)
	byIDFunc        func(uuid.UUID) (user.User, error)
	setDisabledFunc func(uuid.UUID, bool) error
	setRoleFunc     func(uuid.UUID, user.Role) error
	removeByIDFunc  func(uuid.UUID) error
}

func (f *fakeUserRepository) Insert(account user.User) error {
	if f.insertFunc != nil {
		return f.insertFunc(account)
	}

	return nil
}

func (f *fakeUserRepository) List() ([]user.User, error) {
	if f.listFunc != nil {
		return f.listFunc()
	}

	return nil, nil
}

func (f *fakeUserRepository) ByUsername(username string) (user.User, error) {
	if f.byUsernameFunc != nil {
		return f.byUsernameFunc(username)
	}

	return user.User{}, user.ErrUserNotFound
}

func (f *fakeUserRepository) ByID(id uuid.UUID) (user.User, error) {
	if f.byIDFunc != nil {
		return f.byIDFunc(id)
	}

	return user.User{}, user.ErrUserNotFound
}

func (f *fakeUserRepository) SetDisabled(id uuid.UUID, disabled bool) error {
	if f.setDisabledFunc != nil {
		return f.setDisabledFunc(id, disabled)
	}

	return nil
}

func (f *fakeUserRepository) SetRole(id uuid.UUID, role user.Role) error {
	if f.setRoleFunc != nil {
		return f.setRoleFunc(id, role)
	}

	return nil
}

func (f *fakeUserRepository) RemoveByID(id uuid.UUID) error {
	if f.removeByIDFunc != nil {
		return f.removeByIDFunc(id)
	}

	return nil
}

type fakeSessionService struct {
	createFunc   func(uuid.UUID) (string, error)
	validateFunc func(string) (session.Session, error)
	deleteFunc   func(string) error
}

func (f *fakeSessionService) Create(userID uuid.UUID) (string, error) {
	if f.createFunc != nil {
		return f.createFunc(userID)
	}

	return "test-session-token", nil
}

func (f *fakeSessionService) Validate(token string) (session.Session, error) {
	if f.validateFunc != nil {
		return f.validateFunc(token)
	}

	return session.Session{}, nil
}

func (f *fakeSessionService) Delete(token string) error {
	if f.deleteFunc != nil {
		return f.deleteFunc(token)
	}

	return nil
}

func newTestHandler(service endpointService) *handler {
	tmpl := template.Must(
		template.New("index.html").Parse(
			`{{.Error}}|{{len .Endpoints}}|{{if .CSRFToken}}csrf{{end}}`,
		),
	)

	template.Must(
		tmpl.New("endpoint.html").Parse(
			`{{.Endpoint.URL}}|{{.LatencyMS}}|{{if not .LastCheckedAt.IsZero}}{{.LastCheckedAt.Format "15:04:05"}}{{end}}|{{.TLSExpiresAt}}|{{.State}}|{{.StateClass}}|{{.TLSDaysClass}}|{{.Checking}}|{{if .CSRFToken}}csrf{{end}}`,
		),
	)

	template.Must(
		tmpl.New("login.html").Parse(
			`{{.Error}}|{{if .CSRFToken}}csrf{{end}}`,
		),
	)

	template.Must(
		tmpl.New("users.html").Parse(
			`{{.Error}}|{{len .Users}}|{{if .CSRFToken}}csrf{{end}}`,
		),
	)

	return &handler{
		endpoints: service,
		users:     &fakeUserRepository{},
		sessions:  &fakeSessionService{},
		templates: tmpl,
		logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func newCSRFPostRequest(t *testing.T, target string, form url.Values) *http.Request {
	t.Helper()

	tokenRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	tokenRecorder := httptest.NewRecorder()

	token, err := getCSRFToken(tokenRecorder, tokenRequest)
	if err != nil {
		t.Fatalf("getCSRFToken() returned unexpected error: %v", err)
	}

	if form == nil {
		form = url.Values{}
	} else {
		form = form.Clone()
	}

	form.Set(csrfFormField, token)

	req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	cookies := tokenRecorder.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("getCSRFToken() set no cookie")
	}

	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}

	return req
}

func withCurrentUser(r *http.Request, account user.User) *http.Request {
	ctx := context.WithValue(r.Context(), currentUserKey, account)

	return r.WithContext(ctx)
}
