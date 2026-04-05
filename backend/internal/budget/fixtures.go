package budget

import (
	"sync"
	"time"
)

type userState struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

type userRole struct {
	Name    string `json:"name"`
	Created string `json:"created"`
	Updated string `json:"updated"`
}

type user struct {
	ID        int64     `json:"id"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	Email     string    `json:"email"`
	Created   string    `json:"created"`
	Updated   string    `json:"updated"`
	Enabled   bool      `json:"enabled"`
	State     userState `json:"state"`
	Role      userRole  `json:"role"`
}

type group struct {
	Name      string `json:"name"`
	UserCount int    `json:"user_count"`
}

type groupDetails struct {
	Name  string `json:"name"`
	Users []user `json:"users"`
}

type paginatedResponse struct {
	TotalNumber int `json:"total_number"`
	CurrentPage int `json:"current_page"`
	TotalPages  int `json:"total_pages"`
	Items       any `json:"items"`
}

// store holds mutable fixture data, protected by a mutex.
type store struct {
	mu     sync.RWMutex
	users  []user
	groups map[string][]int64 // group name → user IDs
}

var roleCreated = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339)

var allUsers = []user{
	{ID: 1, FirstName: "Mario", LastName: "Rossi", Email: "mario.rossi@acme.com", Created: "2024-01-15T10:00:00Z", Updated: "2025-03-20T14:30:00Z", Enabled: true, State: userState{Name: "active", Enabled: true}, Role: userRole{Name: "manager", Created: roleCreated, Updated: roleCreated}},
	{ID: 2, FirstName: "Giulia", LastName: "Bianchi", Email: "giulia.bianchi@acme.com", Created: "2024-02-10T09:00:00Z", Updated: "2025-02-15T11:00:00Z", Enabled: true, State: userState{Name: "active", Enabled: true}, Role: userRole{Name: "developer", Created: roleCreated, Updated: roleCreated}},
	{ID: 3, FirstName: "Luca", LastName: "Verdi", Email: "luca.verdi@acme.com", Created: "2024-03-05T08:30:00Z", Updated: "2025-01-10T16:45:00Z", Enabled: true, State: userState{Name: "active", Enabled: true}, Role: userRole{Name: "developer", Created: roleCreated, Updated: roleCreated}},
	{ID: 4, FirstName: "Sara", LastName: "Neri", Email: "sara.neri@acme.com", Created: "2024-04-20T11:00:00Z", Updated: "2025-04-01T09:00:00Z", Enabled: true, State: userState{Name: "active", Enabled: true}, Role: userRole{Name: "analyst", Created: roleCreated, Updated: roleCreated}},
	{ID: 5, FirstName: "Marco", LastName: "Ferrari", Email: "marco.ferrari@acme.com", Created: "2024-05-12T14:00:00Z", Updated: "2025-03-28T10:30:00Z", Enabled: true, State: userState{Name: "active", Enabled: true}, Role: userRole{Name: "manager", Created: roleCreated, Updated: roleCreated}},
	{ID: 6, FirstName: "Elena", LastName: "Romano", Email: "elena.romano@acme.com", Created: "2024-06-01T07:45:00Z", Updated: "2025-02-20T13:15:00Z", Enabled: true, State: userState{Name: "active", Enabled: true}, Role: userRole{Name: "developer", Created: roleCreated, Updated: roleCreated}},
	{ID: 7, FirstName: "Andrea", LastName: "Colombo", Email: "andrea.colombo@acme.com", Created: "2024-07-18T10:30:00Z", Updated: "2025-01-25T08:00:00Z", Enabled: true, State: userState{Name: "active", Enabled: true}, Role: userRole{Name: "analyst", Created: roleCreated, Updated: roleCreated}},
	{ID: 8, FirstName: "Chiara", LastName: "Ricci", Email: "chiara.ricci@acme.com", Created: "2024-08-22T09:15:00Z", Updated: "2025-03-15T15:00:00Z", Enabled: true, State: userState{Name: "active", Enabled: true}, Role: userRole{Name: "developer", Created: roleCreated, Updated: roleCreated}},
	{ID: 9, FirstName: "Francesco", LastName: "Marino", Email: "francesco.marino@acme.com", Created: "2024-09-10T13:00:00Z", Updated: "2025-04-02T11:30:00Z", Enabled: true, State: userState{Name: "active", Enabled: true}, Role: userRole{Name: "manager", Created: roleCreated, Updated: roleCreated}},
	{ID: 10, FirstName: "Valentina", LastName: "Greco", Email: "valentina.greco@acme.com", Created: "2024-10-05T08:00:00Z", Updated: "2025-03-30T14:00:00Z", Enabled: true, State: userState{Name: "active", Enabled: true}, Role: userRole{Name: "analyst", Created: roleCreated, Updated: roleCreated}},
}

var db = &store{
	users: allUsers,
	groups: map[string][]int64{
		"Sviluppo":        {2, 3, 6, 8},
		"Marketing":       {4, 7, 10},
		"Vendite":         {1, 5, 9},
		"Amministrazione": {1, 5},
		"Supporto":        {6, 8},
	},
}

func (s *store) getUsers() []user {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.users
}

func (s *store) getUsersByIDs(ids []int64) []user {
	s.mu.RLock()
	defer s.mu.RUnlock()
	idSet := make(map[int64]bool, len(ids))
	for _, id := range ids {
		idSet[id] = true
	}
	var result []user
	for _, u := range s.users {
		if idSet[u.ID] {
			result = append(result, u)
		}
	}
	return result
}

func (s *store) listGroups() []group {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var groups []group
	for name, ids := range s.groups {
		groups = append(groups, group{Name: name, UserCount: len(ids)})
	}
	return groups
}

func (s *store) getGroupDetails(name string) (groupDetails, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids, ok := s.groups[name]
	if !ok {
		return groupDetails{}, false
	}
	users := s.getUsersByIDsLocked(ids)
	return groupDetails{Name: name, Users: users}, true
}

func (s *store) getUsersByIDsLocked(ids []int64) []user {
	idSet := make(map[int64]bool, len(ids))
	for _, id := range ids {
		idSet[id] = true
	}
	var result []user
	for _, u := range s.users {
		if idSet[u.ID] {
			result = append(result, u)
		}
	}
	return result
}

func (s *store) createGroup(name string, userIDs []int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.groups[name] = userIDs
}

func (s *store) editGroup(name string, newName *string, userIDs *[]int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	ids, ok := s.groups[name]
	if !ok {
		return false
	}
	currentName := name
	if newName != nil && *newName != name {
		delete(s.groups, name)
		currentName = *newName
	}
	if userIDs != nil {
		ids = *userIDs
	}
	s.groups[currentName] = ids
	return true
}

func (s *store) deleteGroup(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.groups[name]
	if ok {
		delete(s.groups, name)
	}
	return ok
}
