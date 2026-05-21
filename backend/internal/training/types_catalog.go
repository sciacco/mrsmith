package training

type CatalogCourseWithCounts struct {
	CatalogCourse
	EnrollmentsCurrentYear        int `json:"enrollments_current_year"`
	EnrollmentsCompletedHistorical int `json:"enrollments_completed_historical"`
}

type CatalogListFilters struct {
	SkillArea string
	Vendor    string
	Stato     string
	Search    string
	Year      int
}

type CatalogListResponse struct {
	Courses []CatalogCourseWithCounts `json:"courses"`
}
