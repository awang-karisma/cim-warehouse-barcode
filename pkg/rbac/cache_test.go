package rbac_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/awang-karisma/cim-warehouse-barcode/pkg/rbac"
)

func TestPermissionCache_Expiry(t *testing.T) {
	cache := rbac.NewMemoryCache(50 * time.Millisecond)

	cache.Set("operator", []string{"design:read", "preset:read"})
	perms, ok := cache.Get("operator")
	if !ok || len(perms) != 2 {
		t.Fatalf("expected cache hit with 2 permissions, got ok=%v, len=%d", ok, len(perms))
	}

	time.Sleep(80 * time.Millisecond)

	_, ok = cache.Get("operator")
	if ok {
		t.Errorf("expected cache miss after expiration, got ok=true")
	}
}

func TestPermissionCache_Invalidate(t *testing.T) {
	cache := rbac.NewMemoryCache(5 * time.Minute)

	cache.Set("admin", []string{"design:read", "design:create"})
	perms, ok := cache.Get("admin")
	if !ok || len(perms) != 2 {
		t.Fatalf("expected cache hit, got ok=%v", ok)
	}

	cache.Invalidate("admin")
	_, ok = cache.Get("admin")
	if ok {
		t.Errorf("expected cache miss after Invalidate, got ok=true")
	}
}

func TestPermissionCache_InvalidateAll(t *testing.T) {
	cache := rbac.NewMemoryCache(5 * time.Minute)

	cache.Set("role1", []string{"perm1"})
	cache.Set("role2", []string{"perm2"})

	cache.InvalidateAll()

	if _, ok := cache.Get("role1"); ok {
		t.Errorf("expected role1 miss after InvalidateAll")
	}
	if _, ok := cache.Get("role2"); ok {
		t.Errorf("expected role2 miss after InvalidateAll")
	}
}

func TestPermissionCache_Concurrency(t *testing.T) {
	cache := rbac.NewMemoryCache(500 * time.Millisecond)

	var wg sync.WaitGroup
	for i := range 10 {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			role := fmt.Sprintf("role_%d", id%3)
			for j := range 50 {
				cache.Set(role, []string{"design:read", "preset:read"})
				cache.Get(role)
				if j%10 == 0 {
					cache.Invalidate(role)
				}
				if j%25 == 0 {
					cache.InvalidateAll()
				}
			}
		}(i)
	}
	wg.Wait()
}
