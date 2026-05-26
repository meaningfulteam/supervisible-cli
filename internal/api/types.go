package api

type PublicAPIScope string

type Identity struct {
	KeyID          string           `json:"keyId"`
	KeyName        string           `json:"keyName"`
	OrganizationID string           `json:"organizationId"`
	ActorUserID    string           `json:"actorUserId"`
	Scopes         []PublicAPIScope `json:"scopes"`
}

// Expanded object types (included when ?expand= is used)

type ExpandedUser struct {
	ID    string  `json:"id"`
	Name  *string `json:"name"`
	Email string  `json:"email"`
}

type ExpandedClient struct {
	ID          string `json:"id"`
	CompanyName string `json:"companyName"`
}

type ExpandedProject struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

type ExpandedCapability struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ExpandedTimeOffType struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type User struct {
	ID                  string  `json:"id"`
	OrganizationID      *string `json:"organizationId"`
	Name                *string `json:"name"`
	Email               string  `json:"email"`
	Image               *string `json:"image"`
	CountryCode         *string `json:"countryCode"`
	DefaultAvailability int     `json:"defaultAvailability"`
	IsActive            bool    `json:"isActive"`
	Status              *string `json:"status"`
	ReportsToID         *string `json:"reportsToId"`
	Joined              *string `json:"joined"`
	UserType            string  `json:"userType"`
	CreatedAt           string  `json:"createdAt"`
	UpdatedAt           string  `json:"updatedAt"`
	// Expand fields
	ReportsTo *ExpandedUser `json:"reportsTo,omitempty"`
}

type UpdateUserInput struct {
	Name                *string `json:"name,omitempty"`
	Image               *string `json:"image,omitempty"`
	CountryCode         *string `json:"countryCode,omitempty"`
	DefaultAvailability *int    `json:"defaultAvailability,omitempty"`
	ReportsToID         *string `json:"reportsToId,omitempty"`
}

type ClientResource struct {
	ID               string   `json:"id"`
	OrganizationID   string   `json:"organizationId"`
	CompanyName      string   `json:"companyName"`
	Email            *string  `json:"email"`
	Image            *string  `json:"image"`
	ClientPriority   string   `json:"clientPriority"`
	CountryCode      *string  `json:"countryCode"`
	Website          *string  `json:"website"`
	IsActive         bool     `json:"isActive"`
	CreatedBy        string   `json:"createdBy"`
	CreatedAt        string   `json:"createdAt"`
	UpdatedAt        string   `json:"updatedAt"`
	Categories       []string `json:"categories"`
	AccountManagerID *string  `json:"accountManagerId"`
	// Expand fields
	AccountManager *ExpandedUser `json:"accountManager,omitempty"`
	CreatedByUser  *ExpandedUser `json:"createdByUser,omitempty"`
}

type CreateClientInput struct {
	CompanyName      string   `json:"companyName"`
	Email            *string  `json:"email,omitempty"`
	Image            *string  `json:"image,omitempty"`
	CountryCode      *string  `json:"countryCode,omitempty"`
	Website          *string  `json:"website,omitempty"`
	IsActive         *bool    `json:"isActive,omitempty"`
	ClientPriority   *string  `json:"clientPriority,omitempty"`
	Categories       []string `json:"categories,omitempty"`
	AccountManagerID *string  `json:"accountManagerId,omitempty"`
}

type UpdateClientInput struct {
	CompanyName      *string  `json:"companyName,omitempty"`
	Email            *string  `json:"email,omitempty"`
	Image            *string  `json:"image,omitempty"`
	CountryCode      *string  `json:"countryCode,omitempty"`
	Website          *string  `json:"website,omitempty"`
	IsActive         *bool    `json:"isActive,omitempty"`
	ClientPriority   *string  `json:"clientPriority,omitempty"`
	Categories       []string `json:"categories,omitempty"`
	AccountManagerID *string  `json:"accountManagerId,omitempty"`
}

type Project struct {
	ID               string  `json:"id"`
	ClientID         string  `json:"clientId"`
	Name             string  `json:"name"`
	Objective        *string `json:"objective"`
	StartDate        string  `json:"startDate"`
	EndDate          string  `json:"endDate"`
	ProjectManagerID *string `json:"projectManagerId"`
	Status           string  `json:"status"`
	CreatedAt        string  `json:"createdAt"`
	UpdatedAt        string  `json:"updatedAt"`
	// Expand fields
	Client         *ExpandedClient `json:"client,omitempty"`
	ProjectManager *ExpandedUser   `json:"projectManager,omitempty"`
}

type CreateProjectInput struct {
	Name             string   `json:"name"`
	ClientID         string   `json:"clientId"`
	StartDate        string   `json:"startDate"`
	EndDate          string   `json:"endDate"`
	Objective        *string  `json:"objective,omitempty"`
	ProjectManagerID *string  `json:"projectManagerId,omitempty"`
	Status           *string  `json:"status,omitempty"`
	BillingType      *string  `json:"billingType,omitempty"`
	Amount           *float64 `json:"amount,omitempty"`
	HourlyRate       *float64 `json:"hourlyRate,omitempty"`
}

type UpdateProjectInput struct {
	Name             *string `json:"name,omitempty"`
	Objective        *string `json:"objective,omitempty"`
	StartDate        *string `json:"startDate,omitempty"`
	EndDate          *string `json:"endDate,omitempty"`
	ProjectManagerID *string `json:"projectManagerId,omitempty"`
	Status           *string `json:"status,omitempty"`
}

type Assignment struct {
	ID                string  `json:"id"`
	UserID            string  `json:"userId"`
	ProjectID         string  `json:"projectId"`
	CapabilityID      *string `json:"capabilityId"`
	Date              string  `json:"date"`
	Hours             int     `json:"hours"`
	EffectiveHourRate *string `json:"effectiveHourlyRate"`
	CreatedAt         string  `json:"createdAt"`
	UpdatedAt         string  `json:"updatedAt"`
	// Expand fields
	User       *ExpandedUser       `json:"user,omitempty"`
	Project    *ExpandedProject    `json:"project,omitempty"`
	Capability *ExpandedCapability `json:"capability,omitempty"`
	Client     *ExpandedClient     `json:"client,omitempty"`
}

type AssignmentInput struct {
	UserID       string  `json:"userId"`
	ProjectID    string  `json:"projectId"`
	CapabilityID *string `json:"capabilityId,omitempty"`
	Date         string  `json:"date"`
	Hours        int     `json:"hours"`
}

type AssignmentUpsertInput struct {
	Items []AssignmentInput `json:"items"`
}

type ActualHour struct {
	ID                string  `json:"id"`
	UserID            string  `json:"userId"`
	ProjectID         string  `json:"projectId"`
	CapabilityID      *string `json:"capabilityId"`
	Date              string  `json:"date"`
	Hours             int     `json:"hours"`
	EffectiveHourRate *string `json:"effectiveHourlyRate"`
	CreatedAt         string  `json:"createdAt"`
	UpdatedAt         string  `json:"updatedAt"`
	// Expand fields
	User       *ExpandedUser       `json:"user,omitempty"`
	Project    *ExpandedProject    `json:"project,omitempty"`
	Capability *ExpandedCapability `json:"capability,omitempty"`
}

type ActualHourInput struct {
	UserID       string  `json:"userId"`
	ProjectID    string  `json:"projectId"`
	CapabilityID *string `json:"capabilityId,omitempty"`
	Date         string  `json:"date"`
	Hours        int     `json:"hours"`
}

type ActualHourUpsertInput struct {
	Items []ActualHourInput `json:"items"`
}

type TimeOffRequest struct {
	ID              string  `json:"id"`
	UserID          string  `json:"userId"`
	TimeOffTypeID   string  `json:"timeOffTypeId"`
	StartDate       string  `json:"startDate"`
	EndDate         string  `json:"endDate"`
	Availability    int     `json:"availability"`
	Status          string  `json:"status"`
	Reason          string  `json:"reason"`
	RejectionReason *string `json:"rejectionReason"`
	ApprovedByID    *string `json:"approvedById"`
	ApprovedAt      *string `json:"approvedAt"`
	CreatedAt       string  `json:"createdAt"`
	UpdatedAt       string  `json:"updatedAt"`
	// Expand fields
	User        *ExpandedUser        `json:"user,omitempty"`
	TimeOffType *ExpandedTimeOffType `json:"timeOffType,omitempty"`
	ApprovedBy  *ExpandedUser        `json:"approvedBy,omitempty"`
}

type CreateTimeOffInput struct {
	UserID        string  `json:"userId"`
	TimeOffTypeID string  `json:"timeOffTypeId"`
	StartDate     string  `json:"startDate"`
	EndDate       string  `json:"endDate"`
	Availability  int     `json:"availability"`
	Reason        string  `json:"reason"`
	Status        *string `json:"status,omitempty"`
}

type UpdateTimeOffInput struct {
	TimeOffTypeID *string `json:"timeOffTypeId,omitempty"`
	StartDate     *string `json:"startDate,omitempty"`
	EndDate       *string `json:"endDate,omitempty"`
	Availability  *int    `json:"availability,omitempty"`
	Reason        *string `json:"reason,omitempty"`
}

type RejectTimeOffInput struct {
	RejectionReason string `json:"rejectionReason"`
}
