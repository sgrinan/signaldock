package session

import "context"

type fakeSessionRepository struct {
	insertFunc      func(context.Context, Session) error
	byTokenHashFunc func(context.Context, string) (Session, error)
	deleteFunc      func(context.Context, string) error
}

func (f *fakeSessionRepository) Insert(ctx context.Context, session Session) error {
	if f.insertFunc != nil {
		return f.insertFunc(ctx, session)
	}

	return nil
}

func (f *fakeSessionRepository) ByTokenHash(
	ctx context.Context,
	tokenHash string,
) (Session, error) {
	if f.byTokenHashFunc != nil {
		return f.byTokenHashFunc(ctx, tokenHash)
	}

	return Session{}, ErrSessionNotFound
}

func (f *fakeSessionRepository) Delete(
	ctx context.Context,
	tokenHash string,
) error {
	if f.deleteFunc != nil {
		return f.deleteFunc(ctx, tokenHash)
	}

	return nil
}
