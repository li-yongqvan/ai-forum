package httpapi

import (
	"context"
	"errors"

	"github.com/li-yongqvan/ai-forum/backend/content"
	"github.com/li-yongqvan/ai-forum/backend/moderation"
	"github.com/li-yongqvan/ai-forum/backend/notify"
	"github.com/li-yongqvan/ai-forum/backend/user"
)

// ---- moderation.ContentGateway adapter（#33 S1：组合根把内容域能力接入治理域） ----

type contentGatewayAdapter struct {
	svc content.Service
}

// NewContentGateway 构造 moderation.ContentGateway（供 main 装配 moderation 服务）。
func NewContentGateway(svc content.Service) moderation.ContentGateway {
	return contentGatewayAdapter{svc: svc}
}

// ResolveTarget 解析举报目标：post/comment 取作者与标题/摘要；user 仅回显目标 id。
// 「不存在/已删除」→ moderation.ErrNotFound（服务层据此 404）。
func (a contentGatewayAdapter) ResolveTarget(ctx context.Context, targetType string, targetID int64) (moderation.TargetRef, error) {
	switch targetType {
	case "post":
		p, err := a.svc.GetPostMeta(ctx, targetID)
		if err != nil {
			return moderation.TargetRef{}, mapContentNotFound(err)
		}
		return moderation.TargetRef{AuthorID: p.AuthorID, Title: p.Title}, nil
	case "comment":
		cm, err := a.svc.GetComment(ctx, targetID)
		if err != nil {
			return moderation.TargetRef{}, mapContentNotFound(err)
		}
		return moderation.TargetRef{AuthorID: cm.AuthorID, Title: snippet(cm.Content)}, nil
	default: // user
		return moderation.TargetRef{AuthorID: targetID}, nil
	}
}

// DeletePost/DeleteComment：以处理人身份调用内容域软删（O1-③）；「不存在/已删」当成功（O1-② 幂等重处理）。
func (a contentGatewayAdapter) DeletePost(ctx context.Context, in moderation.DeleteTargetCmd) error {
	err := a.svc.DeletePost(ctx, content.DeletePostCmd{
		OperatorID:   in.OperatorID,
		OperatorRole: in.OperatorRole,
		PostID:       in.TargetID,
	})
	if errors.Is(err, content.ErrPostNotFound) {
		return nil
	}
	return err
}

func (a contentGatewayAdapter) DeleteComment(ctx context.Context, in moderation.DeleteTargetCmd) error {
	err := a.svc.DeleteComment(ctx, content.DeleteCommentCmd{
		OperatorID:   in.OperatorID,
		OperatorRole: in.OperatorRole,
		CommentID:    in.TargetID,
	})
	if errors.Is(err, content.ErrCommentNotFound) {
		return nil
	}
	return err
}

func mapContentNotFound(err error) error {
	if errors.Is(err, content.ErrPostNotFound) || errors.Is(err, content.ErrCommentNotFound) {
		return moderation.ErrNotFound
	}
	return err
}

// snippet 评论内容摘要（列表展示，≤50 rune）。
func snippet(s string) string {
	r := []rune(s)
	if len(r) <= 50 {
		return s
	}
	return string(r[:50]) + "…"
}

// ---- moderation.UserGateway adapter ----

type userGatewayAdapter struct {
	svc user.Service
}

// NewUserGateway 构造 moderation.UserGateway。
func NewUserGateway(svc user.Service) moderation.UserGateway {
	return userGatewayAdapter{svc: svc}
}

// GetUserView 返回治理域可见的最小用户画像。
func (a userGatewayAdapter) GetUserView(ctx context.Context, id int64) (moderation.UserRef, error) {
	u, err := a.svc.GetUser(ctx, id)
	if err != nil {
		if errors.Is(err, user.ErrNotFound) {
			return moderation.UserRef{}, moderation.ErrNotFound
		}
		return moderation.UserRef{}, err
	}
	return moderation.UserRef{ID: u.ID, Username: u.Username}, nil
}

// BanUser 调 user 域封禁，并把 user 域错误翻译为 moderation 哨兵错误（#60，S1：moderation 不 import user）。
func (a userGatewayAdapter) BanUser(ctx context.Context, in moderation.BanUserCmd) error {
	err := a.svc.Ban(ctx, user.BanCmd{OperatorID: in.OperatorID, TargetID: in.TargetID})
	switch {
	case errors.Is(err, user.ErrSelfBan):
		return moderation.ErrSelfBan
	case errors.Is(err, user.ErrCannotBanAdmin):
		return moderation.ErrCannotBanAdmin
	case errors.Is(err, user.ErrAlreadyBanned):
		return moderation.ErrAlreadyBanned
	case errors.Is(err, user.ErrNotFound):
		return moderation.ErrNotFound
	default:
		return err
	}
}

// ---- moderation.Notifier adapter ----

type notifierAdapter struct {
	svc notify.Service
}

// NewNotifier 构造 moderation.Notifier（report_result 通知走 notify 域）。
func NewNotifier(svc notify.Service) moderation.Notifier {
	return notifierAdapter{svc: svc}
}

// CreateNotification 翻译治理域通知命令为 notify 域命令（快照自足，#4 D4）。
func (a notifierAdapter) CreateNotification(ctx context.Context, in moderation.NotificationCmd) error {
	return a.svc.CreateNotification(ctx, notify.CreateNotificationCmd{
		RecipientID: in.RecipientID,
		Type:        in.Type,
		ActorID:     in.ActorID,
		ActorName:   in.ActorName,
		TargetType:  in.TargetType,
		TargetID:    in.TargetID,
		TargetTitle: in.TargetTitle,
	})
}
