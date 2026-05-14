package services

import (
	"context"
	"testing"
	"time"

	"github.com/ablate-ai/RuleFlow/database"
)

// MockSubscriptionRepo 模拟订阅仓储
type MockSubscriptionRepo struct {
	subs map[string]*database.Subscription
}

func NewMockSubscriptionRepo() *MockSubscriptionRepo {
	return &MockSubscriptionRepo{
		subs: make(map[string]*database.Subscription),
	}
}

func (m *MockSubscriptionRepo) Create(ctx context.Context, sub *database.Subscription) error {
	sub.ID = int64(len(m.subs) + 1)
	sub.CreatedAt = time.Now()
	sub.UpdatedAt = time.Now()
	m.subs[sub.Name] = sub
	return nil
}

func stringPtr(s string) *string {
	return &s
}

func (m *MockSubscriptionRepo) GetByName(ctx context.Context, name string) (*database.Subscription, error) {
	sub, ok := m.subs[name]
	if !ok {
		return nil, &testError{"订阅不存在"}
	}
	return sub, nil
}

// testError 测试错误类型
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

func (m *MockSubscriptionRepo) List(ctx context.Context) ([]database.Subscription, error) {
	subs := make([]database.Subscription, 0, len(m.subs))
	for _, sub := range m.subs {
		subs = append(subs, *sub)
	}
	return subs, nil
}

func (m *MockSubscriptionRepo) Update(ctx context.Context, sub *database.Subscription) error {
	sub.UpdatedAt = time.Now()
	m.subs[sub.Name] = sub
	return nil
}

func (m *MockSubscriptionRepo) Delete(ctx context.Context, name string) error {
	delete(m.subs, name)
	return nil
}

func (m *MockSubscriptionRepo) UpdateFetchResult(ctx context.Context, name string, nodeCount int, fetchErr error) error {
	return nil
}

func (m *MockSubscriptionRepo) Exists(ctx context.Context, name string) (bool, error) {
	_, ok := m.subs[name]
	return ok, nil
}

func (m *MockSubscriptionRepo) GetDB() *database.DB {
	return nil
}

// TestSubscriptionService 测试订阅服务
func TestSubscriptionService(t *testing.T) {
	ctx := context.Background()

	// 创建模拟仓储
	repo := NewMockSubscriptionRepo()

	// 由于测试时可能没有 Redis，我们跳过缓存测试
	t.Run("CreateSubscription", func(t *testing.T) {
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

	t.Run("GetSubscription", func(t *testing.T) {
		sub, err := repo.GetByName(ctx, "test-sub")
		if err != nil {
			t.Errorf("获取订阅失败: %v", err)
		}

		if sub.Name != "test-sub" {
			t.Errorf("订阅名称不匹配: got %s, want test-sub", sub.Name)
		}
	})

	t.Run("ListSubscriptions", func(t *testing.T) {
		subs, err := repo.List(ctx)
		if err != nil {
			t.Errorf("列出订阅失败: %v", err)
		}

		if len(subs) != 1 {
			t.Errorf("订阅数量不匹配: got %d, want 1", len(subs))
		}
	})

	t.Run("UpdateSubscription", func(t *testing.T) {
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

		// 验证更新
		updated, _ := repo.GetByName(ctx, "test-sub")
		if updated.Description != "更新后的订阅" {
			t.Errorf("描述未更新: got %s, want 更新后的订阅", updated.Description)
		}
	})

	t.Run("DeleteSubscription", func(t *testing.T) {
		err := repo.Delete(ctx, "test-sub")
		if err != nil {
			t.Errorf("删除订阅失败: %v", err)
		}

		// 验证删除
		_, err = repo.GetByName(ctx, "test-sub")
		if err == nil {
			t.Error("订阅应该已被删除")
		}
	})
}
