package moderation

import (
	"context"
	"sort"
	"strconv"
	"sync"
	"time"
)

// ---- fake Repo（跨 seam 测试 service；ID/时间在 fake 内赋值以模拟 GORM 行为） ----

type fakeRepo struct {
	mu      sync.Mutex
	nextID  int64
	reports map[int64]*Report
	actions []*ModerationAction

	errPendingExists error
	errCreate       error
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{nextID: 1, reports: map[int64]*Report{}}
}

func (f *fakeRepo) CreateReport(ctx context.Context, r *Report) error {
	if f.errCreate != nil {
		return f.errCreate
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	// 模拟 uq_reports_pending：同 reporter/target/type 且 pending → 重复
	for _, ex := range f.reports {
		if ex.ReporterID == r.ReporterID && ex.TargetType == r.TargetType && ex.TargetID == r.TargetID && ex.Status == StatusPending {
			return ErrDuplicatePending
		}
	}
	r.ID = f.nextID
	f.nextID++
	r.CreatedAt = time.Now()
	r.UpdatedAt = time.Now()
	f.reports[r.ID] = r
	return nil
}

func (f *fakeRepo) GetReportByID(ctx context.Context, id int64) (*Report, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.reports[id]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *r
	return &cp, nil
}

func (f *fakeRepo) UpdateReportStatus(ctx context.Context, id int64, status string, handlerID int64, note string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.reports[id]
	if !ok {
		return ErrNotFound
	}
	if r.Status != StatusPending { // F4 条件更新语义
		return ErrNotFound
	}
	now := time.Now()
	r.Status = status
	r.HandlerID = &handlerID
	r.HandledAt = &now
	r.HandlingNote = &note
	r.UpdatedAt = now
	return nil
}

func (f *fakeRepo) ListReports(ctx context.Context, status string, limit, offset int) ([]*Report, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	ids := make([]int64, 0, len(f.reports))
	for id := range f.reports {
		if status != "" && f.reports[id].Status != status {
			continue
		}
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] > ids[j] }) // 模拟 created_at DESC
	if offset > len(ids) {
		offset = len(ids)
	}
	ids = ids[offset:]
	if limit > 0 && limit < len(ids) {
		ids = ids[:limit]
	}
	out := make([]*Report, 0, len(ids))
	for _, id := range ids {
		cp := *f.reports[id]
		out = append(out, &cp)
	}
	return out, nil
}

func (f *fakeRepo) CountReports(ctx context.Context, status string) (int64, error) {
	reps, err := f.ListReports(ctx, status, 0, 0)
	if err != nil {
		return 0, err
	}
	return int64(len(reps)), nil
}

func (f *fakeRepo) PendingExists(ctx context.Context, reporterID int64, targetType string, targetID int64) (bool, error) {
	if f.errPendingExists != nil {
		return false, f.errPendingExists
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.reports {
		if r.ReporterID == reporterID && r.TargetType == targetType && r.TargetID == targetID && r.Status == StatusPending {
			return true, nil
		}
	}
	return false, nil
}

// CountReportsSince 频控窗口计数：同举报人 since 之后所有举报（跨目标、含 dismissed，与 gorm 实现一致，#53）。
func (f *fakeRepo) CountReportsSince(ctx context.Context, reporterID int64, since time.Time) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var n int64
	for _, r := range f.reports {
		if r.ReporterID == reporterID && !r.CreatedAt.Before(since) {
			n++
		}
	}
	return n, nil
}

func (f *fakeRepo) AppendAction(ctx context.Context, a *ModerationAction) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	a.ID = f.nextID
	f.nextID++
	a.CreatedAt = time.Now()
	f.actions = append(f.actions, a)
	return nil
}

// Tx 直通执行（测试只验证行为与事务内写入，不验证回滚机制）。
func (f *fakeRepo) Tx(ctx context.Context, fn func(Repo) error) error {
	return fn(f)
}

// ---- fake 网关（S1 seam 的实现替身） ----

type fakeContent struct {
	mu        sync.Mutex
	refs      map[string]TargetRef // key: "<type>:<id>"
	refErr    error
	deleted   []DeleteTargetCmd
	deleteErr error
}

func newFakeContent() *fakeContent {
	return &fakeContent{refs: map[string]TargetRef{}}
}

func (f *fakeContent) addRef(targetType string, id int64, ref TargetRef) {
	f.refs[targetType+":"+strconv.FormatInt(id, 10)] = ref
}

func (f *fakeContent) ResolveTarget(ctx context.Context, targetType string, targetID int64) (TargetRef, error) {
	if f.refErr != nil {
		return TargetRef{}, f.refErr
	}
	ref, ok := f.refs[targetType+":"+strconv.FormatInt(targetID, 10)]
	if !ok {
		return TargetRef{}, ErrNotFound
	}
	return ref, nil
}

func (f *fakeContent) DeletePost(ctx context.Context, in DeleteTargetCmd) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleted = append(f.deleted, in)
	return nil
}

func (f *fakeContent) DeleteComment(ctx context.Context, in DeleteTargetCmd) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleted = append(f.deleted, in)
	return nil
}

type fakeUsers struct {
	mu    sync.Mutex
	users map[int64]UserRef
}

func newFakeUsers() *fakeUsers {
	return &fakeUsers{users: map[int64]UserRef{}}
}

func (f *fakeUsers) add(id int64, name string) {
	f.users[id] = UserRef{ID: id, Username: name}
}

func (f *fakeUsers) GetUserView(ctx context.Context, id int64) (UserRef, error) {
	u, ok := f.users[id]
	if !ok {
		return UserRef{}, ErrNotFound
	}
	return u, nil
}

type fakeNotifier struct {
	mu    sync.Mutex
	calls []NotificationCmd
	err   error
}

func newFakeNotifier() *fakeNotifier {
	return &fakeNotifier{}
}

func (f *fakeNotifier) CreateNotification(ctx context.Context, in NotificationCmd) error {
	if f.err != nil {
		return f.err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, in)
	return nil
}
