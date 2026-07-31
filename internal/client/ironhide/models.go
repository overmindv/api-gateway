package ironhide

type University struct {
	ID         string  `json:"id,omitempty"`
	Name       string  `json:"name"`
	ShortName  string  `json:"short_name"`
	City       string  `json:"city"`
	Country    string  `json:"country"`
	WebsiteURL string  `json:"website_url"`
	LogoFileID *string `json:"logo_file_id"`
	Status     string  `json:"status"`
	CreatedAt  string  `json:"created_at,omitempty"`
	UpdatedAt  string  `json:"updated_at,omitempty"`
}

type Program struct {
	ID           string  `json:"id,omitempty"`
	UniversityID *string `json:"university_id"`
	Name         string  `json:"name"`
	ShortName    string  `json:"short_name"`
	Faculty      string  `json:"faculty"`
	DegreeLevel  string  `json:"degree_level"`
	StartYear    *int    `json:"start_year"`
	Status       string  `json:"status"`
	CreatedAt    string  `json:"created_at,omitempty"`
	UpdatedAt    string  `json:"updated_at,omitempty"`
}

type Course struct {
	ID          string  `json:"id,omitempty"`
	ProgramID   *string `json:"program_id"`
	Name        string  `json:"name"`
	Slug        string  `json:"slug"`
	Description string  `json:"description"`
	Semester    *int    `json:"semester"`
	YearNumber  *int    `json:"year_number"`
	Status      string  `json:"status"`
	CreatedAt   string  `json:"created_at,omitempty"`
	UpdatedAt   string  `json:"updated_at,omitempty"`
}

type Topic struct {
	ID            string  `json:"id,omitempty"`
	CourseID      *string `json:"course_id"`
	ParentTopicID *string `json:"parent_topic_id"`
	Title         string  `json:"title"`
	Slug          string  `json:"slug"`
	Description   string  `json:"description"`
	OrderIndex    int     `json:"order_index"`
	Difficulty    string  `json:"difficulty"`
	Status        string  `json:"status"`
	CreatedAt     string  `json:"created_at,omitempty"`
	UpdatedAt     string  `json:"updated_at,omitempty"`
}

type TopicTreeNode struct {
	Topic    Topic            `json:"topic"`
	Children []*TopicTreeNode `json:"children"`
}

type TopicPrerequisite struct {
	TopicID             string `json:"topic_id"`
	PrerequisiteTopicID string `json:"prerequisite_topic_id"`
	CreatedAt           string `json:"created_at"`
}

type ListOptions struct {
	Search string
	Status string
	Limit  int
	Offset int
}

type Binding struct {
	UniversityID *string `json:"university_id"`
	ProgramID    *string `json:"program_id"`
	CourseID     *string `json:"course_id"`
	TopicID      *string `json:"topic_id"`
}

type ValidationResult struct {
	Valid bool `json:"valid"`
}

type Actor struct {
	UserID string
	Roles  []string
}

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *Error) Error() string {
	return e.Code + ": " + e.Message
}
