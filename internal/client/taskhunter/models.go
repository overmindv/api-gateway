package taskhunter

type Actor struct {
	UserID string
	Roles  []string
}

type Sources struct {
	TelegramChannels []string `json:"telegram_channels"`
	WebsiteSources   []string `json:"website_sources"`
}

type CreateJobInput struct {
	IdempotencyKey    string   `json:"idempotency_key"`
	TelegramChannels  []string `json:"telegram_channels"`
	PublishedFrom     *string  `json:"published_from"`
	PublishedTo       *string  `json:"published_to"`
	WebsiteURLs       []string `json:"website_urls"`
	MaxItemsPerSource int      `json:"max_items_per_source"`
}

type JobSource struct {
	ID              string `json:"id"`
	Kind            string `json:"kind"`
	SourceID        string `json:"source_id"`
	URL             string `json:"url"`
	Status          string `json:"status"`
	CollectedTotal  int    `json:"collected_total"`
	ImportedTotal   int    `json:"imported_total"`
	DuplicatesTotal int    `json:"duplicates_total"`
	InvalidTotal    int    `json:"invalid_total"`
	ErrorMessage    string `json:"error_message"`
}

type Job struct {
	ID                       string      `json:"id"`
	Trigger                  string      `json:"trigger"`
	RequestedBy              *string     `json:"requested_by"`
	IdempotencyKey           string      `json:"idempotency_key"`
	PublishedFrom            *string     `json:"published_from"`
	PublishedTo              *string     `json:"published_to"`
	MaxItemsPerSource        int         `json:"max_items_per_source"`
	Status                   string      `json:"status"`
	CollectedTotal           int         `json:"collected_total"`
	ImportedTotal            int         `json:"imported_total"`
	DuplicatesTotal          int         `json:"duplicates_total"`
	InvalidTotal             int         `json:"invalid_total"`
	ErrorCount               int         `json:"error_count"`
	ErrorMessage             string      `json:"error_message"`
	NotificationAcknowledged bool        `json:"notification_acknowledged"`
	StartedAt                *string     `json:"started_at"`
	FinishedAt               *string     `json:"finished_at"`
	CreatedAt                string      `json:"created_at"`
	UpdatedAt                string      `json:"updated_at"`
	Sources                  []JobSource `json:"sources"`
}

type JobList struct {
	Items  []Job `json:"items"`
	Limit  int   `json:"limit"`
	Offset int   `json:"offset"`
}

type Error struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	StatusCode int    `json:"-"`
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }
