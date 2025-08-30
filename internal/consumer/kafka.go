package consumer

import (
	"context"
	"fmt"

	"github.com/quiby-ai/common/pkg/events"
	"github.com/quiby-ai/review-vectorizer/config"
	"github.com/quiby-ai/review-vectorizer/internal/service"
)

type VectorizeServiceProcessor struct {
	svc *service.VectorizeService
}

func (p *VectorizeServiceProcessor) Handle(ctx context.Context, payload any, sagaID string) error {
	if evt, ok := payload.(events.VectorizeRequest); ok {
		return p.svc.Handle(ctx, evt, sagaID)
	}
	return fmt.Errorf("invalid payload type for vectorize service")
}

type KafkaConsumer struct {
	consumer *events.KafkaConsumer
}

func NewKafkaConsumer(cfg config.KafkaConfig, svc *service.VectorizeService) *KafkaConsumer {
	consumer := events.NewKafkaConsumer(cfg.Brokers, events.PipelineVectorizeRequest, cfg.GroupID)
	processor := &VectorizeServiceProcessor{svc: svc}
	consumer.SetProcessor(processor)
	return &KafkaConsumer{consumer: consumer}
}

func (kc *KafkaConsumer) Run(ctx context.Context) error {
	return kc.consumer.Run(ctx)
}

func (kc *KafkaConsumer) Close() error {
	return kc.consumer.Close()
}
