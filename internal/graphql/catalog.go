package graphql

import (
	"context"

	"github.com/overmindv/api-gateway/internal/client/entities"
	"github.com/overmindv/api-gateway/internal/graphql/model"
	"github.com/overmindv/api-gateway/internal/middleware"
)

func adminActor(ctx context.Context) (entities.Actor, error) {
	auth, err := middleware.RequireAdmin(ctx)
	if err != nil {
		return entities.Actor{}, err
	}

	return entities.Actor{
		UserID: auth.UserID,
		Roles:  auth.Roles,
	}, nil
}

func catalogOptions(filter *model.CatalogFilter, pagination *model.PaginationInput) entities.ListOptions {
	result := entities.ListOptions{Limit: 50}
	if filter != nil {
		result.Search = stringValue(filter.Search)
		if filter.Status != nil {
			result.Status = filter.Status.String()
		}
	}
	if pagination != nil {
		result.Limit = intValue(pagination.Limit, 50)
		result.Offset = intValue(pagination.Offset, 0)
	}

	return result
}

func universityModel(item entities.University) *model.University {
	return &model.University{ID: item.ID, Name: item.Name, ShortName: item.ShortName, City: item.City, Country: item.Country, WebsiteURL: item.WebsiteURL, LogoFileID: item.LogoFileID, Status: model.CatalogStatus(item.Status), CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
}

func programModel(item entities.Program) *model.Program {
	return &model.Program{ID: item.ID, UniversityID: item.UniversityID, Name: item.Name, ShortName: item.ShortName, Faculty: item.Faculty, DegreeLevel: model.DegreeLevel(item.DegreeLevel), StartYear: item.StartYear, Status: model.CatalogStatus(item.Status), CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
}

func courseModel(item entities.Course) *model.Course {
	return &model.Course{ID: item.ID, ProgramID: item.ProgramID, Name: item.Name, Slug: item.Slug, Description: item.Description, Semester: item.Semester, YearNumber: item.YearNumber, Status: model.CatalogStatus(item.Status), CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
}

func topicModel(item entities.Topic) *model.Topic {
	return &model.Topic{ID: item.ID, CourseID: item.CourseID, ParentTopicID: item.ParentTopicID, Title: item.Title, Slug: item.Slug, Description: item.Description, OrderIndex: item.OrderIndex, Difficulty: model.TopicDifficulty(item.Difficulty), Status: model.CatalogStatus(item.Status), CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
}

func prerequisiteModel(item entities.TopicPrerequisite) *model.TopicPrerequisite {
	return &model.TopicPrerequisite{TopicID: item.TopicID, PrerequisiteTopicID: item.PrerequisiteTopicID, CreatedAt: item.CreatedAt}
}

func treeModel(item *entities.TopicTreeNode) *model.TopicTreeNode {
	children := make([]*model.TopicTreeNode, 0, len(item.Children))
	for _, child := range item.Children {
		children = append(children, treeModel(child))
	}

	return &model.TopicTreeNode{Topic: topicModel(item.Topic), Children: children}
}

func statusValue(value *model.CatalogStatus) string {
	if value == nil {
		return "draft"
	}

	return value.String()
}

func degreeValue(value *model.DegreeLevel) string {
	if value == nil {
		return "other"
	}

	return value.String()
}

func difficultyValue(value *model.TopicDifficulty) string {
	if value == nil {
		return "basic"
	}

	return value.String()
}
