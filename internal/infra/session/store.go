package session

import "sync"

type Store struct {
	sessions    *map[string]string
	sessionsMux *sync.RWMutex
}

func NewStore(sessions *map[string]string, sessionsMux *sync.RWMutex) Store {
	return Store{sessions: sessions, sessionsMux: sessionsMux}
}

func (s Store) GetUserEmail(token string) string {
	s.sessionsMux.RLock()
	defer s.sessionsMux.RUnlock()
	return (*s.sessions)[token]
}

func (s Store) SetUserSession(token, email string) {
	s.sessionsMux.Lock()
	defer s.sessionsMux.Unlock()
	(*s.sessions)[token] = email
}

func (s Store) DeleteUserSession(token string) {
	s.sessionsMux.Lock()
	defer s.sessionsMux.Unlock()
	delete(*s.sessions, token)
}
