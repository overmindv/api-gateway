package tasks

import "io"

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
	TopicID     *string           `json:"topic_id"`
	Title       string            `json:"title"`
	Statement   string            `json:"statement"`
	TaskType    string            `json:"task_type"`
	Difficulty  string            `json:"difficulty"`
	Options     []TaskOptionInput `json:"options"`
	Tags        []string          `json:"tags"`
	Examples    []TaskExample     `json:"examples"`
	Constraints []string          `json:"constraints"`
	Source      *TaskSource       `json:"source,omitempty"`
}

type TaskExample struct {
	Input       string `json:"input"`
	Output      string `json:"output"`
	Explanation string `json:"explanation"`
}

type TaskSource struct {
	SourceID    string  `json:"source_id"`
	SourceName  string  `json:"source_name"`
	SourceURL   string  `json:"source_url"`
	PublishedAt *string `json:"published_at"`
}

type TaskOption struct {
	ID        string `json:"id"`
	Text      string `json:"text"`
	Position  int    `json:"position"`
	IsCorrect *bool  `json:"is_correct,omitempty"`
}

type Task struct {
	ID            string        `json:"id"`
	Status        string        `json:"status"`
	TaskVersionID string        `json:"task_version_id"`
	VersionNumber int           `json:"version_number"`
	TopicID       *string       `json:"topic_id"`
	Title         string        `json:"title"`
	Statement     string        `json:"statement"`
	TaskType      string        `json:"task_type"`
	Difficulty    string        `json:"difficulty"`
	Options       []TaskOption  `json:"options"`
	Tags          []string      `json:"tags"`
	Examples      []TaskExample `json:"examples"`
	Constraints   []string      `json:"constraints"`
	Source        *TaskSource   `json:"source"`
	CreatedAt     string        `json:"created_at"`
	UpdatedAt     string        `json:"updated_at"`
}

type CandidateFilter struct {
	Status     string
	SourceID   string
	Difficulty string
	Limit      int
	Offset     int
}

type CandidateReview struct {
	ExpectedRevision int           `json:"expected_revision"`
	TopicID          *string       `json:"topic_id"`
	Title            string        `json:"title"`
	Statement        string        `json:"statement"`
	Difficulty       string        `json:"difficulty"`
	Tags             []string      `json:"tags"`
	Examples         []TaskExample `json:"examples"`
	Constraints      []string      `json:"constraints"`
}

type Candidate struct {
	ID                string        `json:"id"`
	Status            string        `json:"status"`
	Revision          int           `json:"revision"`
	ExternalID        string        `json:"external_id"`
	SourceID          string        `json:"source_id"`
	SourceName        string        `json:"source_name"`
	SourceURL         string        `json:"source_url"`
	SourcePublishedAt *string       `json:"source_published_at"`
	RetrievedAt       string        `json:"retrieved_at"`
	CollectionJobID   string        `json:"collection_job_id"`
	TopicID           *string       `json:"topic_id"`
	Title             string        `json:"title"`
	Statement         string        `json:"statement"`
	Difficulty        string        `json:"difficulty"`
	Tags              []string      `json:"tags"`
	Examples          []TaskExample `json:"examples"`
	Constraints       []string      `json:"constraints"`
	ApprovedTaskID    *string       `json:"approved_task_id"`
	RejectionReason   string        `json:"rejection_reason"`
	CreatedAt         string        `json:"created_at"`
	UpdatedAt         string        `json:"updated_at"`
}

type CandidateList struct {
	Items  []Candidate `json:"items"`
	Limit  int         `json:"limit"`
	Offset int         `json:"offset"`
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

type CodeSubmissionInput struct {
	TaskVersionID  string
	IdempotencyKey string
	Language       string
	FileName       string
	File           io.Reader
	// SourceCode задаёт консольный вариант решения; если задан, File и FileName игнорируются.
	SourceCode *string
}

type ExecutionPhaseResult struct {
	ExitCode    *int   `json:"exit_code,omitempty"`
	Stdout      string `json:"stdout"`
	Stderr      string `json:"stderr"`
	DurationMS  int64  `json:"duration_ms"`
	MemoryBytes int64  `json:"memory_bytes"`
}

type ExecutionTestResult struct {
	TestID      string `json:"test_id"`
	Verdict     string `json:"verdict"`
	Stdout      string `json:"stdout"`
	Stderr      string `json:"stderr"`
	DurationMS  int64  `json:"duration_ms"`
	MemoryBytes int64  `json:"memory_bytes"`
}

type ExecutionFailure struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type CodeSubmission struct {
	ID                string                `json:"id"`
	UserID            string                `json:"user_id"`
	TaskID            string                `json:"task_id"`
	TaskVersionID     string                `json:"task_version_id"`
	TaskVersionNumber int                   `json:"task_version_number"`
	ExecutionID       string                `json:"execution_id"`
	CorrelationID     string                `json:"correlation_id"`
	Language          string                `json:"language"`
	SourceFileName    string                `json:"source_file_name"`
	SourceCode        string                `json:"source_code"`
	Status            string                `json:"status"`
	Verdict           *string               `json:"verdict,omitempty"`
	Compilation       *ExecutionPhaseResult `json:"compilation,omitempty"`
	Execution         *ExecutionPhaseResult `json:"execution,omitempty"`
	Tests             []ExecutionTestResult `json:"tests"`
	Failure           *ExecutionFailure     `json:"failure,omitempty"`
	CreatedAt         string                `json:"created_at"`
	UpdatedAt         string                `json:"updated_at"`
	CompletedAt       *string               `json:"completed_at,omitempty"`
}

type CodeSubmissionList struct {
	Items  []CodeSubmission `json:"items"`
	Limit  int              `json:"limit"`
	Offset int              `json:"offset"`
}

type Error struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	StatusCode int    `json:"-"`
}

// Error возвращает безопасное описание ошибки tasks.
func (e *Error) Error() string {
	return e.Code + ": " + e.Message
}
