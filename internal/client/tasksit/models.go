package tasksit

type Actor struct {
	UserID string
	Roles  []string
}

type TaskFilter struct {
	Status     string
	TaskType   string
	Difficulty string
	TopicID    *string
	Limit      int
	Offset     int
}

type TaskOptionInput struct {
	Text      string `json:"text"`
	IsCorrect bool   `json:"is_correct"`
}

type TaskInput struct {
	TopicID    *string           `json:"topic_id"`
	Title      string            `json:"title"`
	Statement  string            `json:"statement"`
	TaskType   string            `json:"task_type"`
	Difficulty string            `json:"difficulty"`
	Options    []TaskOptionInput `json:"options"`
}

type TaskOption struct {
	ID        string `json:"id"`
	Text      string `json:"text"`
	Position  int    `json:"position"`
	IsCorrect *bool  `json:"is_correct,omitempty"`
}

type Task struct {
	ID            string       `json:"id"`
	Status        string       `json:"status"`
	TaskVersionID string       `json:"task_version_id"`
	VersionNumber int          `json:"version_number"`
	TopicID       *string      `json:"topic_id"`
	Title         string       `json:"title"`
	Statement     string       `json:"statement"`
	TaskType      string       `json:"task_type"`
	Difficulty    string       `json:"difficulty"`
	Options       []TaskOption `json:"options"`
	CreatedAt     string       `json:"created_at"`
	UpdatedAt     string       `json:"updated_at"`
}

type TaskSummary struct {
	ID            string  `json:"id"`
	Status        string  `json:"status"`
	TaskVersionID string  `json:"task_version_id"`
	VersionNumber int     `json:"version_number"`
	TopicID       *string `json:"topic_id"`
	Title         string  `json:"title"`
	TaskType      string  `json:"task_type"`
	Difficulty    string  `json:"difficulty"`
	CreatedAt     string  `json:"created_at"`
	UpdatedAt     string  `json:"updated_at"`
}

type TaskList struct {
	Items  []TaskSummary `json:"items"`
	Limit  int           `json:"limit"`
	Offset int           `json:"offset"`
}

type SubmissionInput struct {
	TaskVersionID     string   `json:"task_version_id"`
	IdempotencyKey    string   `json:"idempotency_key"`
	SelectedOptionIDs []string `json:"selected_option_ids"`
}

type Submission struct {
	ID                  string   `json:"id"`
	UserID              string   `json:"user_id"`
	TaskID              string   `json:"task_id"`
	TaskVersionID       string   `json:"task_version_id"`
	TaskVersionNumber   int      `json:"task_version_number"`
	SelectedOptionIDs   []string `json:"selected_option_ids"`
	CorrectOptionIDs    []string `json:"correct_option_ids"`
	Correct             bool     `json:"correct"`
	Verdict             string   `json:"verdict"`
	TaskUpdated         bool     `json:"task_updated"`
	LatestTaskVersionID string   `json:"latest_task_version_id"`
	LatestVersionNumber int      `json:"latest_version_number"`
	CreatedAt           string   `json:"created_at"`
}

type SubmissionList struct {
	Items  []Submission `json:"items"`
	Limit  int          `json:"limit"`
	Offset int          `json:"offset"`
}

type Error struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	StatusCode int    `json:"-"`
}

// Error возвращает безопасное описание ошибки tasks-it.
func (e *Error) Error() string {
	return e.Code + ": " + e.Message
}
