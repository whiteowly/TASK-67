package components

import "github.com/campusrec/campusrec/internal/model"

// PageContext holds common data passed to all templates.
type PageContext struct {
	FacilityName string
	User         *model.UserPublic
}

func (c PageContext) IsLoggedIn() bool {
	return c.User != nil
}

func (c PageContext) HasRole(role string) bool {
	if c.User == nil {
		return false
	}
	for _, r := range c.User.Roles {
		if r == role {
			return true
		}
	}
	return false
}

func (c PageContext) IsAdmin() bool {
	return c.HasRole("administrator")
}

func (c PageContext) IsStaff() bool {
	return c.HasRole("staff")
}

func (c PageContext) IsModerator() bool {
	return c.HasRole("moderator")
}
