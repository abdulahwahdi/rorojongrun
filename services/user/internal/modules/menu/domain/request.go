package domain

// RequestMenu is the payload to create / update a menu node
type RequestMenu struct {
	Key          string `json:"key"`
	Label        string `json:"label"`
	Path         string `json:"path"`
	Icon         string `json:"icon"`
	SortOrder    int    `json:"sortOrder"`
	ParentID     *int   `json:"parentId"`
	PermissionID *int   `json:"permissionId"`
}
