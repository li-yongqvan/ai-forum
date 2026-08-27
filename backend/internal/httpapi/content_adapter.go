package httpapi

import (
	"context"

	"github.com/li-yongqvan/ai-forum/backend/content"
	"github.com/li-yongqvan/ai-forum/backend/notify"
)

// ---- content.Notifier adapter（#72：组合根把通知能力接入内容域；照 moderation.Notifier 先例） ----

type mentionNotifierAdapter struct {
	svc notify.Service
}

// NewContentNotifier 构造 content.Notifier（提及通知走 notify 域）。
func NewContentNotifier(svc notify.Service) content.Notifier {
	return mentionNotifierAdapter{svc: svc}
}

// NotifyMention 翻译内容域提及通知命令为 notify 域命令（快照自足，#4 D4；取地址传值安全，逃逸分析放堆）。
func (a mentionNotifierAdapter) NotifyMention(ctx context.Context, in content.MentionNotificationCmd) error {
	return a.svc.CreateNotification(ctx, notify.CreateNotificationCmd{
		RecipientID: in.RecipientID,
		Type:        "mention",
		ActorID:     &in.ActorID,
		ActorName:   &in.ActorName,
		ActorAvatar: in.ActorAvatar,
		TargetType:  &in.TargetType,
		TargetID:    &in.TargetID,
		TargetTitle: &in.TargetTitle,
	})
}
