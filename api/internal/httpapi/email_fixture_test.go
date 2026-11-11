package httpapi

import "sync"

// No real emails are sent by integration tests.
type recordingMailer struct {
	mu    sync.Mutex
	codes map[string]string
}

func (m *recordingMailer) Configured() bool { return true }
func (m *recordingMailer) SendCode(to, code, purpose string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.codes == nil {
		m.codes = make(map[string]string)
	}
	m.codes[to+":"+purpose] = code
	return nil
}
func (m *recordingMailer) code(to, purpose string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.codes[to+":"+purpose]
}
