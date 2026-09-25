package domain

// RequestUser is the payload to create a user
type RequestCreateUser struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	FullName string `json:"fullName"`
	Password string `json:"password"`
	Status   string `json:"status"`
	RoleIDs  []int  `json:"roleIds"`
}

// RequestUpdateUser is the payload to update a user's profile / status (password has its own endpoint)
type RequestUpdateUser struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	FullName string `json:"fullName"`
	Status   string `json:"status"`
}

// RequestPassword is the payload to set a password
type RequestPassword struct {
	Password string `json:"password"`
}

// RequestRoleIDs is a set of role ids
type RequestRoleIDs struct {
	RoleIDs []int `json:"roleIds"`
}

// RequestRoleID is a single role id
type RequestRoleID struct {
	RoleID int `json:"roleId"`
}
