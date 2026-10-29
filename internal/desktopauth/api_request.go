package desktopauth

import "net/http"

// AuthorizeRequest is a trusted Go-only capability. Neither the request nor its
// Authorization header is exposed by a Wails binding or authentication DTO.
func (m *Manager) AuthorizeRequest(generation uint64, request *http.Request) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.requireIdentityLocked(generation); err != nil {
		return err
	}
	if request == nil || request.URL == nil || request.URL.Scheme != "https" || request.URL.User != nil {
		return ErrConfiguration
	}
	request.Header.Set("Authorization", "Bearer "+m.session.accessToken)
	return nil
}
