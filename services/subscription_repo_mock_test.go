package services

import (
	"context"
	"testing"

	"github.com/ablate-ai/RuleFlow/database"
)

func TestMockSubscriptionRepoCRUD(t *testing.T) {
	ctx := context.Background()
	repo := NewMockSubscriptionRepo()

	t.Run("Create", func(t *testing.T) {
		sub := &database.Subscription{
			Name:        "test-sub",
			URL:         stringPtr("https://example.com/sub"),
			Enabled:     true,
			Description: "测试订阅",
		}

		err := repo.Create(ctx, sub)
		if err != nil {
			t.Errorf("创建订阅失败: %v", err)
		}
		if sub.ID == 0 {
			t.Error("订阅 ID 应该被设置")
		}
	})

	t.Run("Get", func(t *testing.T) {
		sub, err := repo.GetByName(ctx, "test-sub")
		if err != nil {
			t.Errorf("获取订阅失败: %v", err)
		}
		if sub.Name != "test-sub" {
			t.Errorf("订阅名称不匹配: got %s, want test-sub", sub.Name)
		}
	})

	t.Run("List", func(t *testing.T) {
		subs, err := repo.List(ctx)
		if err != nil {
			t.Errorf("列出订阅失败: %v", err)
		}
		if len(subs) != 1 {
			t.Errorf("订阅数量不匹配: got %d, want 1", len(subs))
		}
	})

	t.Run("Update", func(t *testing.T) {
		sub := &database.Subscription{
			Name:        "test-sub",
			URL:         stringPtr("https://example.com/new-sub"),
			Enabled:     false,
			Description: "更新后的订阅",
		}

		err := repo.Update(ctx, sub)
		if err != nil {
			t.Errorf("更新订阅失败: %v", err)
		}

		updated, _ := repo.GetByName(ctx, "test-sub")
		if updated.Description != "更新后的订阅" {
			t.Errorf("描述未更新: got %s, want 更新后的订阅", updated.Description)
		}
	})

	t.Run("Delete", func(t *testing.T) {
		err := repo.Delete(ctx, "test-sub")
		if err != nil {
			t.Errorf("删除订阅失败: %v", err)
		}
		_, err = repo.GetByName(ctx, "test-sub")
		if err == nil {
			t.Error("订阅应该已被删除")
		}
	})
}
