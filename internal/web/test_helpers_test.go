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
	addFunc        func(context.Context, string) (endpoint.Endpoint, error)
	listFunc       func(context.Context) ([]endpoint.Endpoint, error)
	byIDFunc       func(context.Context, uuid.UUID) (endpoint.Endpoint, error)
	removeByIDFunc func(context.Context, uuid.UUID) error
	refreshFunc    func(context.Context, uuid.UUID) (endpoint.CheckResult, error)
}

var _ endpointService = (*fakeEndpointService)(nil)

func (f *fakeEndpointService) Add(ctx context.Context, rawURL string) (endpoint.Endpoint, error) {
	if f.addFunc != nil {
		return f.addFunc(ctx, rawURL)
	}

	return endpoint.Endpoint{}, nil
}

func (f *fakeEndpointService) List(ctx context.Context) ([]endpoint.Endpoint, error) {
	if f.listFunc != nil {
		return f.listFunc(ctx)
	}

	return nil, nil
}

func (f *fakeEndpointService) ByID(ctx context.Context, id uuid.UUID) (endpoint.Endpoint, error) {
	if f.byIDFunc != nil {
		return f.byIDFunc(ctx, id)
	}

	return endpoint.Endpoint{}, endpoint.ErrEndpointNotFound
}

func (f *fakeEndpointService) RemoveByID(ctx context.Context, id uuid.UUID) error {
	if f.removeByIDFunc != nil {
		return f.removeByIDFunc(ctx, id)
	}

	return nil
}

func (f *fakeEndpointService) Refresh(ctx context.Context, id uuid.UUID) (endpoint.CheckResult, error) {
	if f.refreshFunc != nil {
		return f.refreshFunc(ctx, id)
	}

	return endpoint.CheckResult{}, nil
}

type fakeUserRepository struct {
	insertFunc       func(context.Context, user.User) error
	listFunc         func(context.Context) ([]user.User, error)
	byUsernameFunc   func(context.Context, string) (user.User, error)
	byIDFunc         func(context.Context, uuid.UUID) (user.User, error)
	updateAccessFunc func(context.Context, uuid.UUID, user.Role, bool) error
	removeByIDFunc   func(context.Context, uuid.UUID) error
}

func (f *fakeUserRepository) Insert(ctx context.Context, account user.User) error {
	if f.insertFunc != nil {
		return f.insertFunc(ctx, account)
	}

	return nil
}

func (f *fakeUserRepository) List(ctx context.Context) ([]user.User, error) {
	if f.listFunc != nil {
		return f.listFunc(ctx)
	}

	return nil, nil
}

func (f *fakeUserRepository) ByUsername(ctx context.Context, username string) (user.User, error) {
	if f.byUsernameFunc != nil {
		return f.byUsernameFunc(ctx, username)
	}

	return user.User{}, user.ErrUserNotFound
}

func (f *fakeUserRepository) ByID(ctx context.Context, id uuid.UUID) (user.User, error) {
	if f.byIDFunc != nil {
		return f.byIDFunc(ctx, id)
	}

	return user.User{}, user.ErrUserNotFound
}

func (f *fakeUserRepository) UpdateAccess(ctx context.Context, id uuid.UUID, role user.Role, disabled bool) error {
	if f.updateAccessFunc != nil {
		return f.updateAccessFunc(ctx, id, role, disabled)
	}

	return nil
}

func (f *fakeUserRepository) RemoveByID(ctx context.Context, id uuid.UUID) error {
	if f.removeByIDFunc != nil {
		return f.removeByIDFunc(ctx, id)
	}

	return nil
}

type fakeSessionService struct {
	createFunc   func(context.Context, uuid.UUID) (string, error)
	validateFunc func(context.Context, string) (session.Session, error)
	deleteFunc   func(context.Context, string) error
}

func (f *fakeSessionService) Create(ctx context.Context, userID uuid.UUID) (string, error) {
	if f.createFunc != nil {
		return f.createFunc(ctx, userID)
	}

	return "test-session-token", nil
}

func (f *fakeSessionService) Validate(ctx context.Context, token string) (session.Session, error) {
	if f.validateFunc != nil {
		return f.validateFunc(ctx, token)
	}

	return session.Session{}, nil
}

func (f *fakeSessionService) Delete(ctx context.Context, token string) error {
	if f.deleteFunc != nil {
		return f.deleteFunc(ctx, token)
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
