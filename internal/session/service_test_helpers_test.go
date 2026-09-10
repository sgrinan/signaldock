package session

type fakeSessionRepository struct {
	insertFunc      func(Session) error
	byTokenHashFunc func(string) (Session, error)
	deleteFunc      func(string) error
}

func (f *fakeSessionRepository) Insert(session Session) error {
	if f.insertFunc != nil {
		return f.insertFunc(session)
	}

	return nil
}

func (f *fakeSessionRepository) ByTokenHash(tokenHash string) (Session, error) {
	if f.byTokenHashFunc != nil {
		return f.byTokenHashFunc(tokenHash)
	}

	return Session{}, ErrSessionNotFound
}

func (f *fakeSessionRepository) Delete(tokenHash string) error {
	if f.deleteFunc != nil {
		return f.deleteFunc(tokenHash)
	}

	return nil
}
