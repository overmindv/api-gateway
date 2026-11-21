package graphql

import (
	"context"
	"time"

	contentclient "github.com/overmindv/api-gateway/internal/client/content"
	"github.com/overmindv/api-gateway/internal/graphql/model"
	pb "github.com/overmindv/content/pkg/api/content"
)

func (r *queryResolver) PublishedMaterialAttachmentDownload(ctx context.Context, id, revisionID, fileID string) (*model.MediaDownload, error) {
	client, actor, err := r.materialService(ctx)
	if err != nil {
		return nil, err
	}
	for _, value := range []*string{&id, &revisionID, &fileID} {
		*value, err = materialID(*value)
		if err != nil {
			return nil, err
		}
	}
	result, err := client.DownloadPublishedAttachment(ctx, actor, &pb.GetPublishedAttachmentDownloadRequest{
		MaterialId: id, RevisionId: revisionID, FileId: fileID,
	})
	if err != nil {
		return nil, err
	}
	expiresAt := result.GetExpiresAt()
	if result.GetUrl() == "" || expiresAt.CheckValid() != nil || !expiresAt.AsTime().After(time.Now()) {
		return nil, &contentclient.Error{Code: "SERVICE_UNAVAILABLE"}
	}

	return &model.MediaDownload{URL: result.GetUrl(), ExpiresAt: expiresAt.AsTime().Format(time.RFC3339Nano)}, nil
}
