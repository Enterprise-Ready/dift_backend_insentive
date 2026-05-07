package natsadmin

import (
	"context"
	"fmt"
	"strings"
	"time"

	adminport "coupon-service/internal/interface/admin"
	"coupon-service/internal/model"

	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	topicCouponCreated = "admin.coupon.created"
	topicCouponUpdated = "admin.coupon.updated"
	topicCouponDeleted = "admin.coupon.deleted"
)

type AdminCouponConsumer struct {
	js       nats.JetStreamContext
	stream   string
	subject  string
	durable  string
	svc      adminport.CouponCommandPort
	fetchMax int
}

func NewAdminCouponConsumer(
	js nats.JetStreamContext,
	stream string,
	subject string,
	durable string,
	svc adminport.CouponCommandPort,
) *AdminCouponConsumer {
	return &AdminCouponConsumer{
		js:       js,
		stream:   stream,
		subject:  subject,
		durable:  durable,
		svc:      svc,
		fetchMax: 16,
	}
}

func (c *AdminCouponConsumer) Start(ctx context.Context) error {
	if c.js == nil || c.svc == nil {
		return nil
	}

	sub, err := c.js.PullSubscribe(
		c.subject,
		c.durable,
		nats.BindStream(c.stream),
	)
	if err != nil {
		return fmt.Errorf("subscribe admin coupon events: %w", err)
	}

	for {
		if ctx.Err() != nil {
			return nil
		}

		msgs, err := sub.Fetch(c.fetchMax, nats.MaxWait(2*time.Second))
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			continue
		}

		for _, msg := range msgs {
			if err := c.handleMessage(ctx, msg.Subject, msg.Data); err != nil {
				_ = msg.Nak()
				continue
			}
			_ = msg.Ack()
		}
	}
}

func (c *AdminCouponConsumer) handleMessage(ctx context.Context, subject string, payload []byte) error {
	wire := &structpb.Struct{}
	if err := proto.Unmarshal(payload, wire); err != nil {
		return err
	}
	data := wire.AsMap()

	switch subject {
	case topicCouponCreated:
		return c.svc.CreateCoupon(ctx, mapCreatedCoupon(data))
	case topicCouponUpdated:
		code := getString(data, "code")
		if code == "" {
			return nil
		}
		return c.svc.UpdateCoupon(ctx, mapUpdatedCoupon(code, data))
	case topicCouponDeleted:
		code := getString(data, "code")
		if code == "" {
			code = getString(data, "coupon_id")
		}
		if code == "" {
			return nil
		}
		return c.svc.DeactivateCoupon(ctx, code)
	default:
		return nil
	}
}

func mapCreatedCoupon(data map[string]any) model.Coupon {
	now := time.Now().UTC()
	return model.Coupon{
		Code:          getString(data, "code"),
		DiscountType:  mapDiscountType(getString(data, "type")),
		DiscountValue: getFloat(data, "value"),
		MinOrder:      getFloat(data, "min_order"),
		MaxDiscount:   getFloat(data, "max_discount"),
		MaxUsage:      int32(getInt(data, "quota")),
		ValidFrom:     now.Add(-1 * time.Minute),
		ValidTo:       parseTime(data, "expires_at", now.Add(30*24*time.Hour)),
		Active:        true,
	}
}

func mapUpdatedCoupon(code string, data map[string]any) model.Coupon {
	now := time.Now().UTC()
	return model.Coupon{
		Code:          code,
		DiscountType:  mapDiscountType(getString(data, "type")),
		DiscountValue: getFloat(data, "value"),
		MinOrder:      getFloat(data, "min_order"),
		MaxDiscount:   getFloat(data, "max_discount"),
		MaxUsage:      int32(getInt(data, "quota")),
		ValidFrom:     now.Add(-1 * time.Minute),
		ValidTo:       parseTime(data, "expires_at", now.Add(30*24*time.Hour)),
		Active:        true,
	}
}

func mapDiscountType(v string) model.DiscountType {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "percentage", "percent":
		return model.DiscountPercent
	case "flat", "fixed", "cashback", "delivery", "free_ride":
		return model.DiscountFixed
	default:
		return model.DiscountFixed
	}
}

func getString(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(s)
}

func getFloat(m map[string]any, key string) float64 {
	v, ok := m[key]
	if !ok || v == nil {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int32:
		return float64(n)
	case int64:
		return float64(n)
	default:
		return 0
	}
}

func getInt(m map[string]any, key string) int {
	return int(getFloat(m, key))
}

func parseTime(m map[string]any, key string, fallback time.Time) time.Time {
	s := getString(m, key)
	if s == "" {
		return fallback
	}
	ts, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return fallback
	}
	return ts.UTC()
}
