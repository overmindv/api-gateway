package media

type Actor struct {
	UserID string
	Roles  []string
}

type CreateUploadInput struct {
	OriginalName string `json:"original_name"`
	ContentType  string `json:"content_type"`
	SizeBytes    int64  `json:"size_bytes"`
	Checksum     string `json:"checksum_sha256"`
	Purpose      string `json:"purpose"`
	Visibility   string `json:"visibility"`
}

type UploadTarget struct {
	FileID      string            `json:"file_id"`
	Mode        string            `json:"mode"`
	URL         string            `json:"url"`
	Fields      map[string]string `json:"fields"`
	Headers     map[string]string `json:"headers"`
	MultipartID string            `json:"multipart_upload_id"`
	PartSize    int64             `json:"part_size"`
	ExpiresAt   string            `json:"expires_at"`
}

type UploadPart struct {
	PartNumber int    `json:"part_number"`
	URL        string `json:"url"`
	ExpiresAt  string `json:"expires_at"`
}

type CompletedPart struct {
	PartNumber int    `json:"part_number"`
	ETag       string `json:"etag"`
}

type File struct {
	ID                  string  `json:"id"`
	OwnerUserID         string  `json:"owner_user_id"`
	Purpose             string  `json:"purpose"`
	Visibility          string  `json:"visibility"`
	OriginalName        string  `json:"original_name"`
	DeclaredContentType string  `json:"declared_content_type"`
	DetectedContentType string  `json:"detected_content_type"`
	SizeBytes           int64   `json:"size_bytes"`
	Status              string  `json:"status"`
	FailureCode         string  `json:"failure_code"`
	PublicURL           string  `json:"public_url"`
	CreatedAt           string  `json:"created_at"`
	UpdatedAt           string  `json:"updated_at"`
	DeletedAt           *string `json:"deleted_at"`
}

type FileList struct {
	Items  []File `json:"items"`
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
}

type Download struct {
	URL       string `json:"url"`
	ExpiresAt string `json:"expires_at"`
}

type PublicFile struct {
	FileID string            `json:"file_id"`
	URLs   map[string]string `json:"urls"`
}

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Error возвращает строковое представление upstream ошибки Media.
func (e *Error) Error() string { return e.Code + ": " + e.Message }
