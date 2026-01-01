package logctx

import (
	"context"
	"sdsyslog/internal/global"
)

// Append new tag to tag list.
// It performs copy-on-write to preserve immutability
func AppendCtxTag(ctx context.Context, newTag string) (newCtx context.Context) {
	old := GetTagList(ctx)

	// copy old slice, prevents mutation of parent context
	tags := append(append([]string(nil), old...), newTag)

	newCtx = context.WithValue(ctx, global.LogTagsKey, tags)
	return
}

// Removes last index of tag list.
// Also uses copy-on-write
func RemoveLastCtxTag(ctx context.Context) (newCtx context.Context) {
	old := GetTagList(ctx)

	// copy old slice
	tags := append([]string(nil), old...)

	if len(tags) > 0 {
		tags = tags[:len(tags)-1]
	}

	newCtx = context.WithValue(ctx, global.LogTagsKey, tags)
	return
}

// Overwrites entire tag list with given list
func OverwriteCtxTag(ctx context.Context, newList []string) (newCtx context.Context) {
	newCtx = context.WithValue(ctx, global.LogTagsKey, newList)
	return
}

// Extracts tag list from context or returns empty array
func GetTagList(ctx context.Context) (tags []string) {
	tags, validAssert := ctx.Value(global.LogTagsKey).([]string)
	if !validAssert {
		tags = []string{}
		return
	}
	return
}
