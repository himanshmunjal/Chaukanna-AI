// Package session runs live calls.
//
// A Session is created for every phone call (or simulator run). It receives
// transcripts, feeds them to the detector, asks Evon for a verdict, speaks
// warnings through Timbre, alerts the family and streams everything to the
// dashboard.
//
// Files in this package:
//
//	manager.go   - creates sessions and keeps track of active ones (this file)
//	session.go   - shared session logic: transcripts, saving, broadcasting, ending
//	guardian.go  - Guardian Mode: silent listener that warns when a scam is detected
//	screen.go    - Screen Mode: the bot answers unknown callers and reports back
//	legs.go      - the Leg interface (how we play audio into a call) + simulator leg
package session

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"gorm.io/gorm"

	"chaukanna/internal/alerts"
	"chaukanna/internal/clips"
	"chaukanna/internal/detector"
	"chaukanna/internal/hub"
	"chaukanna/internal/models"
	"chaukanna/internal/reasoner"
	"chaukanna/internal/tts"
)

// Mode says how Chaukanna takes part in a call.
type Mode string

const (
	// Guardian: the user merged Chaukanna into a live call. We listen silently
	// and speak only to warn.
	Guardian Mode = "guardian"
	// Screen: an unknown caller was forwarded to Chaukanna. The bot talks to
	// them, then calls the user back with a summary.
	Screen Mode = "screen"
)

// Callbacker places a phone call that plays an audio file (Twilio implements it).
type Callbacker interface {
	Enabled() bool
	CallWithAudio(ctx context.Context, to, audioURL string) error
}

// Deps are the services a session uses.
type Deps struct {
	DB       *gorm.DB
	Hub      *hub.Hub
	Evon     *reasoner.Client
	TTS      *tts.Client
	Alerts   *alerts.Notifier
	Clips    *clips.Store
	Callback Callbacker // may be nil
}

// Settings are the tunable values from .env.
type Settings struct {
	Detector         detector.Config
	WarningCooldown  time.Duration
	StoreTranscripts string // all | flagged | none
	ScreenMaxTurns   int
	PublicURL        string
	DashboardURL     string
}

// Manager creates sessions and tracks the active ones.
type Manager struct {
	deps     Deps
	settings Settings

	mu     sync.Mutex
	active map[uint]*Session
}

// NewManager creates a Manager.
func NewManager(deps Deps, settings Settings) *Manager {
	return &Manager{deps: deps, settings: settings, active: map[uint]*Session{}}
}

// StartParams describes a new call.
type StartParams struct {
	Provider       string // twilio | exotel | simulator
	ProviderCallID string
	Mode           Mode
	CallerNumber   string // the unknown caller (screen mode); empty in guardian mode
	UserPhone      string // the protected user's phone (used to find their profile)
	Leg            Leg    // how to play audio into the call

	// Owner marks a public demo call by an anonymous visitor. Such a call uses a made-up
	// person with no family, sends no alerts, doesn't touch the scam-number list, and
	// uses no LLM: nothing about it can reach a real person or spend this server's budget.
	Owner    string
	Language string      // the demo person's language
	TTS      *tts.Client // voice for this call; nil = the server's
}

// Start creates a session, saves the call and starts its background loop.
func (m *Manager) Start(p StartParams) (*Session, error) {
	if p.Mode != Screen {
		p.Mode = Guardian
	}
	var user models.ProtectedUser
	var err error
	if p.Owner != "" {
		user, err = m.demoUser()
		if p.Language != "" {
			user.Language = p.Language
		}
		p.CallerNumber = "" // keeps the known-scam list out of reach of visitors
	} else {
		user, err = m.findUser(p.UserPhone)
	}
	if err != nil {
		return nil, err
	}

	call := models.Call{
		Provider:        p.Provider,
		ProviderCallID:  p.ProviderCallID,
		Mode:            string(p.Mode),
		ProtectedUserID: &user.ID,
		CallerNumber:    models.NormalizePhone(p.CallerNumber),
		State:           string(detector.Listening),
		Language:        user.Language,
		DemoOwner:       p.Owner,
		StartedAt:       time.Now(),
	}
	if err := m.deps.DB.Create(&call).Error; err != nil {
		return nil, err
	}
	call.ProtectedUser = &user
	if p.Owner != "" {
		m.deps.Hub.SetOwner(call.ID, p.Owner)
	}

	ctx, cancel := context.WithCancel(context.Background())
	s := &Session{
		m:         m,
		call:      call,
		user:      user,
		mode:      p.Mode,
		leg:       p.Leg,
		startedAt: call.StartedAt,
		ctx:       ctx,
		cancel:    cancel,
		det:       detector.New(m.settings.Detector),
		tts:       p.TTS,
	}
	if s.tts == nil {
		s.tts = m.deps.TTS
	}
	s.llm = m.deps.Evon
	if p.Owner != "" {
		s.llm = nil // nil is a valid, disabled client
	}

	if n, ok := m.knownScamNumber(call.CallerNumber); ok {
		s.knownScam = &n
	}

	m.mu.Lock()
	m.active[call.ID] = s
	m.mu.Unlock()

	log.Printf("session %d: started (%s, %s mode, user %s)", call.ID, p.Provider, p.Mode, user.Name)
	m.deps.Hub.Broadcast(hub.Event{Type: "call_started", CallID: call.ID, Data: call})

	go s.run()
	switch {
	case s.knownScam != nil:
		go s.handleKnownScam()
	case p.Mode == Screen:
		go s.screenGreet()
	}
	return s, nil
}

// ResolveMode decides how to handle an incoming call to the Chaukanna number:
//   - the caller IS a protected user  -> they merged us into a call -> Guardian
//   - anyone else                     -> a forwarded unknown call   -> Screen
func (m *Manager) ResolveMode(from, forwardedFrom string) (mode Mode, userPhone, caller string) {
	from = models.NormalizePhone(from)
	if _, ok := m.userByPhone(from); ok {
		return Guardian, from, ""
	}
	return Screen, models.NormalizePhone(forwardedFrom), from
}

// ActiveCallIDs lists calls in progress that this owner may see.
func (m *Manager) ActiveCallIDs(owner string) []uint {
	m.mu.Lock()
	defer m.mu.Unlock()
	ids := make([]uint, 0, len(m.active))
	for id, s := range m.active {
		if s.call.DemoOwner == owner {
			ids = append(ids, id)
		}
	}
	return ids
}

// ActiveDemoCalls counts demo calls in progress: this owner's, and everyone's.
func (m *Manager) ActiveDemoCalls(owner string) (mine, all int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.active {
		if s.call.DemoOwner != "" {
			all++
			if s.call.DemoOwner == owner {
				mine++
			}
		}
	}
	return mine, all
}

// demoUser finds or creates the made-up person used on demo calls.
func (m *Manager) demoUser() (models.ProtectedUser, error) {
	var u models.ProtectedUser
	err := m.deps.DB.Where(models.ProtectedUser{Phone: models.DemoUserPhone}).
		Attrs(models.ProtectedUser{Name: models.DemoUserName, Language: "hi"}).FirstOrCreate(&u).Error
	u.FamilyPhone, u.TelegramChatID = "", "" // whatever is stored, a demo call has no family
	return u, err
}

// Get returns the live session of a call in progress.
func (m *Manager) Get(id uint) (*Session, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.active[id]
	return s, ok
}

func (m *Manager) remove(id uint) {
	m.mu.Lock()
	delete(m.active, id)
	m.mu.Unlock()
}

// findUser finds the protected user by phone, falling back to the first user.
// If there are no users at all, a demo user is created so the simulator works
// on a fresh install.
func (m *Manager) findUser(phone string) (models.ProtectedUser, error) {
	if u, ok := m.userByPhone(phone); ok {
		return u, nil
	}
	var first []models.ProtectedUser
	if m.deps.DB.Where("phone <> ?", models.DemoUserPhone).Order("id").Limit(1).Find(&first); len(first) == 1 {
		return first[0], nil
	}
	u, err := m.demoUser()
	if err != nil {
		return u, errors.New("no protected user found and could not create a demo user: " + err.Error())
	}
	return u, nil
}

// userByPhone looks up a protected user. (Find instead of First: "not found" is
// a normal answer here, not an error worth logging.)
func (m *Manager) userByPhone(phone string) (models.ProtectedUser, bool) {
	phone = models.NormalizePhone(phone)
	if phone == "" {
		return models.ProtectedUser{}, false
	}
	var users []models.ProtectedUser
	m.deps.DB.Where("phone = ?", phone).Limit(1).Find(&users)
	if len(users) == 0 {
		return models.ProtectedUser{}, false
	}
	return users[0], true
}

// knownScamNumber returns the record for a caller number that was caught scamming before.
func (m *Manager) knownScamNumber(phone string) (models.ScamNumber, bool) {
	phone = models.NormalizePhone(phone)
	if phone == "" {
		return models.ScamNumber{}, false
	}
	var found []models.ScamNumber
	m.deps.DB.Where("phone = ?", phone).Limit(1).Find(&found)
	if len(found) == 0 {
		return models.ScamNumber{}, false
	}
	return found[0], true
}

// recordScamNumber remembers a scam caller's number (or counts another block).
func (m *Manager) recordScamNumber(phone, reason string, callID uint, blocked bool) {
	phone = models.NormalizePhone(phone)
	if phone == "" {
		return
	}
	n, exists := m.knownScamNumber(phone)
	if !exists {
		n = models.ScamNumber{Phone: phone, Source: "auto"}
	}
	if blocked {
		n.Blocked++
	} else {
		n.Reports++
		n.LastReason = reason
	}
	n.LastCallID = &callID
	if err := m.deps.DB.Save(&n).Error; err != nil {
		log.Printf("scam numbers: could not save %s: %v", phone, err)
		return
	}
	log.Printf("scam numbers: %s now has %d report(s), %d block(s)", phone, n.Reports, n.Blocked)
}
