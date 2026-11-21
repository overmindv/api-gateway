package graphql

import (
	"context"
	"strings"

	gqlgen "github.com/99designs/gqlgen/graphql"
)

const queryComplexityLimit = 25000

// costSchema задаёт веса списков без изменения сгенерированного gqlgen-кода.
type costSchema struct{ gqlgen.ExecutableSchema }

func (s costSchema) Complexity(_ context.Context, object, field string, child int, args map[string]any) (int, bool) {
	if (object == "MaterialRevision" || object == "MaterialChangeRevision") && field == "source" {
		return 4000 + child, true
	}
	if object == objectQuery && (field == "materialChangeDiff" || field == "publishedMaterialAttachmentDownload" || field == "materialModeration") {
		return 6500 + child, true
	}
	if object == objectQuery && field == "materialChangeProposal" {
		return 4000 + child, true
	}
	if object == objectQuery && field == "materialReviewStatus" {
		return 4000 + child, true
	}
	if field == "body" && (object == "MaterialReviewThread" || object == "MaterialReviewReply") {
		return 40 + child, true
	}
	if object == objectMutation {
		switch field {
		case "mergeMaterialChange", "rebaseMaterialChange", "restoreMaterialRevision", "setMaterialModeration":
			return 6000 + child, true
		case "submitMaterialReview", "createMaterialReviewThread", "replyToMaterialReviewThread", "setMaterialReviewThreadResolved":
			return 2000 + child, true
		}
	}
	definition := s.Schema().Types[object]
	if definition == nil {
		return 1 + child, true
	}
	f := definition.Fields.ForName(field)
	if f == nil {
		return 1 + child, true
	}
	if f.Arguments.ForName("pagination") != nil {
		size := 50
		if page, ok := args["pagination"].(map[string]any); ok {
			if n, err := gqlgen.UnmarshalInt(page["limit"]); err == nil && n > 0 {
				size = n
			}
		}

		return weightedCost(child, size), true
	}
	if f.Arguments.ForName("limit") != nil {
		size := 50
		if n, err := gqlgen.UnmarshalInt(args["limit"]); err == nil && n > 0 {
			size = n
		}

		return weightedCost(child, size), true
	}
	if object == "TopicTreeNode" && field == "children" {
		// Дерево загружается одним запросом; его общий вес учтён в Query.topicTree.
		// Повторное умножение на каждом уровне ошибочно блокирует обычный каталог.
		return 1 + child, true
	}
	if f.Type.Elem != nil {
		// Пагинация уже учтена на корневом поле connection.
		if field == "items" && (strings.HasSuffix(object, "List") || strings.HasSuffix(object, "Connection") || strings.HasSuffix(object, "Page")) {
			return 1 + child, true
		}
		if object == "SearchResults" {
			return 1 + child, true
		}
		size := 20
		if object == objectQuery {
			size = 100
		}

		return weightedCost(child, size), true
	}

	return 1 + child, true
}

func weightedCost(child, size int) int {
	if child > queryComplexityLimit {
		return child
	}
	if child < 1 {
		child = 1
	}
	if size > queryComplexityLimit/child {
		return queryComplexityLimit + 1
	}

	return 1 + size*child
}
