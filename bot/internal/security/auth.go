package security

type Authorizer struct {
	admin map[int64]struct{}
}

func NewAuthorizer(adminIDs []int64) Authorizer {
	out := Authorizer{
		admin: make(map[int64]struct{}, len(adminIDs)),
	}
	for _, id := range adminIDs {
		out.admin[id] = struct{}{}
	}
	return out
}

func (a Authorizer) IsAdmin(userID int64) bool {
	_, ok := a.admin[userID]
	return ok
}
