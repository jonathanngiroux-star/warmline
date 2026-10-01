package sendgrid

import (
	"testing"

	"github.com/jonathanngiroux-star/warmline/internal/webhooks"
)

// The migrate dry-run report and the live webhook normalizer must share
// one mapping table. This test pins the full canonical surface so any
// change to the shared table consciously updates both consumers.
func TestWebhookMappingTableShared(t *testing.T) {
	reported := map[string]bool{} // events the dry-run reports unmapped
	for _, ev := range sendgridWebhookEvents {
		if _, ok := webhooks.SendgridEventKind(ev); ok {
			continue
		}
		reported[ev] = true
	}
	if len(reported) != 2 || !reported["dropped"] || !reported["group_resubscribe"] {
		t.Errorf("unmapped set = %v, want exactly {dropped, group_resubscribe}", reported)
	}
	// And the normalizer must accept exactly what the report accepts.
	for _, ev := range []string{"processed", "delivered", "open", "click", "bounce", "deferred", "spam_report", "unsubscribe", "group_unsubscribe"} {
		if _, ok := webhooks.SendgridEventKind(ev); !ok {
			t.Errorf("normalizer missing %q which the report counts as mapped", ev)
		}
	}
}

// sendgridWebhookEvents is the full event-name surface a SendGrid
// account webhook can subscribe to (Event Webhook API).
var sendgridWebhookEvents = []string{
	"processed", "delivered", "open", "click", "bounce", "deferred",
	"dropped", "spam_report", "unsubscribe", "group_unsubscribe", "group_resubscribe",
}

func TestReportUsesSharedTable(t *testing.T) {
	// The dry-run report's unmapped entries must equal exactly the
	// fixture's webhook events absent from the shared table — proof the
	// report reads the same table the normalizer does.
	f, err := ParseFile(fixtureRel)
	if err != nil {
		t.Fatal(err)
	}
	r, err := f.BuildReport()
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Unmapped) != 2 {
		t.Fatalf("unmapped = %d, want 2", len(r.Unmapped))
	}
	got := map[string]bool{r.Unmapped[0].Key: true, r.Unmapped[1].Key: true}
	if !got["webhook.event.dropped"] || !got["webhook.event.group_resubscribe"] {
		t.Errorf("unmapped keys = %v", got)
	}
}
