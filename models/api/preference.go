package api

// AlertPreference says where one kind of alert goes: by email, in 9itchi, both or neither.
type AlertPreference struct {
	// Kind is applications, matches, closing or tips.
	Kind  string `json:"kind" binding:"required,oneof=applications matches closing tips"`
	Email *bool  `json:"email" binding:"required"`
	InApp *bool  `json:"in_app" binding:"required"`
}

// UpdateAlertPreferencesRequest is the validated body of UpdateAlertPreferences. Kinds left out keep their choice.
type UpdateAlertPreferencesRequest struct {
	Items []AlertPreference `json:"items" binding:"required,min=1,max=4,dive"`
}

// AlertPreferenceResponse is the choice for one kind.
type AlertPreferenceResponse struct {
	Kind  string `json:"kind"`
	Email bool   `json:"email"`
	InApp bool   `json:"in_app"`
}

// AlertPreferencesResponse lists the four kinds, in the order the screen shows them.
type AlertPreferencesResponse struct {
	Items []AlertPreferenceResponse `json:"items"`
}
