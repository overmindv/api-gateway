package graphql

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	pb "github.com/overmindv/content/pkg/api/content"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const attachmentFileID = "00000000-0000-4000-8000-000000000111"
const attachmentFixtureURL = "https://objects.invalid/media/material?signature=fixture"

func (materialRPCFixture) GetPublishedAttachmentDownload(ctx context.Context, req *pb.GetPublishedAttachmentDownloadRequest) (*pb.GetPublishedAttachmentDownloadResponse, error) {
	if materialRPCActor(ctx) != materialReader || req.GetMaterialId() != materialFixtureID ||
		req.GetRevisionId() != materialPublishedID || req.GetFileId() != attachmentFileID {
		return nil, status.Error(codes.NotFound, "private upstream details")
	}

	return &pb.GetPublishedAttachmentDownloadResponse{
		Url: attachmentFixtureURL, ExpiresAt: timestamppb.New(time.Now().Add(30 * time.Second)),
	}, nil
}

func checkPublishedAttachmentGraphQL(t *testing.T, run func(string, string, ...string)) {
	t.Helper()

	field := `publishedMaterialAttachmentDownload(materialId:%q,revisionId:%q,fileId:%q){ url expiresAt }`
	query := func(revisionID, fileID string) string {
		return "{ " + fmt.Sprintf(field, materialFixtureID, revisionID, fileID) + " }"
	}
	valid := query(materialPublishedID, attachmentFileID)
	run("", valid, `"code":"UNAUTHENTICATED"`)
	run(materialReader, valid, attachmentFixtureURL)
	run(materialReader, query(materialDraftID, attachmentFileID), `"code":"NOT_FOUND"`)
	run(materialReader, query(materialPublishedID, materialDraftID), `"code":"NOT_FOUND"`)
	run(materialReader, query(materialPublishedID, "%2f"), `"code":"INVALID_ARGUMENT"`)
	aliases := make([]string, 4)
	for i := range aliases {
		aliases[i] = fmt.Sprintf("link%d: ", i) + fmt.Sprintf(field, materialFixtureID, materialPublishedID, attachmentFileID)
	}
	run(materialReader, "{ "+strings.Join(aliases, " ")+" }", `"code":"COMPLEXITY_LIMIT_EXCEEDED"`)
}
