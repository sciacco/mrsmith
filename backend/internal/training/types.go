package training

type Principal struct {
	Subject       string   `json:"subject"`
	Email         string   `json:"email"`
	Name          string   `json:"name"`
	Roles         []string `json:"roles"`
	IsPeopleAdmin bool     `json:"isPeopleAdmin"`
}

type Employee struct {
	ID        string `json:"id"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Email     string `json:"email"`
	Status    string `json:"status"`
}

type MeResponse struct {
	Principal         Principal `json:"principal"`
	Employee          *Employee `json:"employee"`
	OnboardingPending bool      `json:"onboardingPending"`
}

type CertificationRow struct {
	AwardID           string `json:"awardId"`
	EmployeeName      string `json:"employeeName"`
	EmployeeEmail     string `json:"employeeEmail,omitempty"`
	CertificationCode string `json:"certificationCode"`
	CertificationName string `json:"certificationName"`
	Outcome           string `json:"outcome"`
	AwardedOn         string `json:"awardedOn"`
	ExpiresOn         string `json:"expiresOn,omitempty"`
	CurrentStatus     string `json:"currentStatus"`
	ValidationSource  string `json:"validationSource"`
	DocumentID        string `json:"documentId,omitempty"`
	DocumentFilename  string `json:"documentFilename,omitempty"`
	DocumentValidated bool   `json:"documentValidated"`
}

type ExpiringCertificationRow struct {
	EmployeeName      string `json:"employeeName"`
	EmployeeEmail     string `json:"employeeEmail"`
	CertificationCode string `json:"certificationCode"`
	CertificationName string `json:"certificationName"`
	ExpiresOn         string `json:"expiresOn"`
	DaysToExpiry      int    `json:"daysToExpiry"`
}

type LookupItem struct {
	ID                  string `json:"id"`
	Label               string `json:"label"`
	Active              bool   `json:"active"`
	ComplianceRelated   bool   `json:"complianceRelated,omitempty"`
	ComplianceFramework string `json:"complianceFramework,omitempty"`
}

type LookupResponse struct {
	Employees      []LookupItem `json:"employees"`
	Teams          []LookupItem `json:"teams"`
	Vendors        []LookupItem `json:"vendors"`
	SkillAreas     []LookupItem `json:"skillAreas"`
	Courses        []LookupItem `json:"courses"`
	Certifications []LookupItem `json:"certifications"`
}

type ActionResponse struct {
	OK     bool   `json:"ok"`
	ID     string `json:"id,omitempty"`
	Status string `json:"status,omitempty"`
}

type PersonUpdateInput struct {
	FirstName string  `json:"firstName"`
	LastName  string  `json:"lastName"`
	Email     string  `json:"email"`
	Status    string  `json:"status"`
	TeamID    *string `json:"teamId"`
	Notes     string  `json:"notes,omitempty"`
	// DirectoryExempt: se presente, imposta la gestione manuale (esclusione
	// dalla sincronizzazione anagrafica).
	DirectoryExempt *bool `json:"directoryExempt,omitempty"`
}

type PersonCreateInput struct {
	FirstName string  `json:"firstName"`
	LastName  string  `json:"lastName"`
	Email     string  `json:"email"`
	Status    string  `json:"status"`
	TeamID    *string `json:"teamId"`
	Notes     string  `json:"notes,omitempty"`
}

type VendorInput struct {
	Name    string `json:"name"`
	Website string `json:"website,omitempty"`
	Notes   string `json:"notes,omitempty"`
	Active  *bool  `json:"active,omitempty"`
}

type TeamInput struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Active      *bool  `json:"active,omitempty"`
}

type SkillAreaInput struct {
	Code     string `json:"code"`
	Name     string `json:"name"`
	ParentID string `json:"parentId,omitempty"`
	// CustomGroupID: gruppo locale che raccoglie gli appartenenti all'area;
	// e la platea delle regole con kind skill_area (migrazione 131).
	CustomGroupID string `json:"customGroupId,omitempty"`
	Description   string `json:"description,omitempty"`
	Active        *bool  `json:"active,omitempty"`
}

type CertificationInput struct {
	Code                  string `json:"code"`
	Name                  string `json:"name"`
	IssuerVendorID        string `json:"issuerVendorId,omitempty"`
	SkillAreaID           string `json:"skillAreaId,omitempty"`
	TypicalValidityMonths *int   `json:"typicalValidityMonths,omitempty"`
	// AttestedLevel: livello che la certificazione attesta sulla sua area
	// (scala 0-5 delle valutazioni), facoltativo.
	AttestedLevel *int   `json:"attestedLevel,omitempty"`
	Description   string `json:"description,omitempty"`
	Active        *bool  `json:"active,omitempty"`
}

// CourseVisibilityInput e la platea di visibilita del corso, con lo stesso
// vocabolario delle platee delle regole. Assente = riservato a People;
// «all» = pubblico; per kind «people» i destinatari sono employeeIds.
type CourseVisibilityInput struct {
	Kind        string   `json:"kind"` // all|team|skill_area|custom_group|people
	ID          string   `json:"id,omitempty"`
	EmployeeIDs []string `json:"employeeIds,omitempty"`
}

type CourseInput struct {
	Title               string   `json:"title"`
	VendorID            string   `json:"vendorId,omitempty"`
	SkillAreaIDs        []string `json:"skillAreaIds,omitempty"`
	LeadsToCertID       string   `json:"leadsToCertId,omitempty"`
	DeliveryMode        string   `json:"deliveryMode,omitempty"`
	ProviderKind        string   `json:"providerKind,omitempty"`
	DefaultHours        *int     `json:"defaultHours,omitempty"`
	DefaultCost         *float64 `json:"defaultCost,omitempty"`
	CourseURL           string   `json:"courseUrl,omitempty"`
	Description         string   `json:"description,omitempty"`
	ComplianceRelated   bool     `json:"complianceRelated"`
	Mandatory           bool     `json:"mandatory"`
	ComplianceFramework string   `json:"complianceFramework,omitempty"`
	Tags                []string `json:"tags,omitempty"`
	Active              *bool    `json:"active,omitempty"`
	Notes               string   `json:"notes,omitempty"`
	ReminderText        string   `json:"reminderText,omitempty"`
	ReminderAt          string   `json:"reminderAt,omitempty"` // YYYY-MM-DD
	// TrainerIDs: formatori interni designati in istruttoria; alla
	// declinazione in evento diventano il valore di partenza dell'evento.
	TrainerIDs []string               `json:"trainerIds,omitempty"`
	Visibility *CourseVisibilityInput `json:"visibility,omitempty"`
}

type AwardInput struct {
	EmployeeID            string `json:"employeeId"`
	CertificationID       string `json:"certificationId"`
	EnrollmentID          string `json:"enrollmentId,omitempty"`
	Outcome               string `json:"outcome"`
	AwardedOn             string `json:"awardedOn"`
	ExpiresOn             string `json:"expiresOn,omitempty"`
	ValidationSource      string `json:"validationSource,omitempty"`
	ExternalCredentialID  string `json:"externalCredentialId,omitempty"`
	ExternalCredentialURL string `json:"externalCredentialUrl,omitempty"`
	Notes                 string `json:"notes,omitempty"`
	Reason                string `json:"reason,omitempty"`
}

type AwardUpdateInput struct {
	Outcome          string  `json:"outcome"`
	AwardedOn        string  `json:"awardedOn"`
	ExpiresOn        string  `json:"expiresOn,omitempty"`
	ValidationSource string  `json:"validationSource,omitempty"`
	Notes            *string `json:"notes,omitempty"`
}

type DocumentMetadata struct {
	ID                   string `json:"id"`
	EnrollmentID         string `json:"enrollmentId,omitempty"`
	CertificationAwardID string `json:"certificationAwardId,omitempty"`
	Filename             string `json:"filename"`
	SHA256               string `json:"sha256"`
	MIME                 string `json:"mime"`
	SizeBytes            int64  `json:"sizeBytes"`
	UploadedAt           string `json:"uploadedAt"`
	Validated            bool   `json:"validated"`
}

type JobRunResponse struct {
	OK bool `json:"ok"`
}
