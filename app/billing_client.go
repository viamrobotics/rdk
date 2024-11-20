package app

import (
	"context"
	"sync"

	pb "go.viam.com/api/app/v1"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.viam.com/rdk/logging"
)

// UsageCostType specifies the type of usage cost.
type UsageCostType int32

const (
	// UsageCostTypeUnspecified is an unspecified usage cost type.
	UsageCostTypeUnspecified UsageCostType = iota
	// UsageCostTypeDataUpload represents the usage cost from data upload.
	UsageCostTypeDataUpload
	// UsageCostTypeDataEgress represents the usage cost from data egress.
	UsageCostTypeDataEgress
	// UsageCostTypeRemoteControl represents the usage cost from remote control.
	UsageCostTypeRemoteControl
	// UsageCostTypeStandardCompute represents the usage cost from standard compute.
	UsageCostTypeStandardCompute
	// UsageCostTypeCloudStorage represents the usage cost from cloud storage.
	UsageCostTypeCloudStorage
	// UsageCostTypeBinaryDataCloudStorage represents the usage cost from binary data cloud storage.
	UsageCostTypeBinaryDataCloudStorage
	// UsageCostTypeOtherCloudStorage represents the usage cost from other cloud storage.
	UsageCostTypeOtherCloudStorage
	// UsageCostTypePerMachine represents the usage cost per machine.
	UsageCostTypePerMachine
)

func usageCostTypeFromProto(costType pb.UsageCostType) UsageCostType {
	switch costType {
	case pb.UsageCostType_USAGE_COST_TYPE_UNSPECIFIED:
		return UsageCostTypeUnspecified
	case pb.UsageCostType_USAGE_COST_TYPE_DATA_UPLOAD:
		return UsageCostTypeDataUpload
	case pb.UsageCostType_USAGE_COST_TYPE_DATA_EGRESS:
		return UsageCostTypeDataEgress
	case pb.UsageCostType_USAGE_COST_TYPE_REMOTE_CONTROL:
		return UsageCostTypeRemoteControl
	case pb.UsageCostType_USAGE_COST_TYPE_STANDARD_COMPUTE:
		return UsageCostTypeStandardCompute
	case pb.UsageCostType_USAGE_COST_TYPE_CLOUD_STORAGE:
		return UsageCostTypeCloudStorage
	case pb.UsageCostType_USAGE_COST_TYPE_BINARY_DATA_CLOUD_STORAGE:
		return UsageCostTypeBinaryDataCloudStorage
	case pb.UsageCostType_USAGE_COST_TYPE_OTHER_CLOUD_STORAGE:
		return UsageCostTypeOtherCloudStorage
	case pb.UsageCostType_USAGE_COST_TYPE_PER_MACHINE:
		return UsageCostTypePerMachine
	default:
		return UsageCostTypeUnspecified
	}
}

// UsageCost contains the cost and cost type.
type UsageCost struct {
	ResourceType UsageCostType
	Cost         float64
}

func usageCostFromProto(cost *pb.UsageCost) *UsageCost {
	return &UsageCost{
		ResourceType: usageCostTypeFromProto(cost.ResourceType),
		Cost:         cost.Cost,
	}
}

// ResourceUsageCosts holds the usage costs with discount information.
type ResourceUsageCosts struct {
	UsageCosts           []*UsageCost
	Discount             float64
	TotalWithDiscount    float64
	TotalWithoutDiscount float64
}

func resourceUsageCostsFromProto(costs *pb.ResourceUsageCosts) *ResourceUsageCosts {
	var usageCosts []*UsageCost
	for _, cost := range costs.UsageCosts {
		usageCosts = append(usageCosts, usageCostFromProto(cost))
	}
	return &ResourceUsageCosts{
		UsageCosts:           usageCosts,
		Discount:             costs.Discount,
		TotalWithDiscount:    costs.TotalWithDiscount,
		TotalWithoutDiscount: costs.TotalWithoutDiscount,
	}
}

// SourceType is the type of source from which a cost is coming from.
type SourceType int32

const (
	// SourceTypeUnspecified represents an unspecified source type.
	SourceTypeUnspecified SourceType = iota
	// SourceTypeOrg represents an organization.
	SourceTypeOrg
	// SourceTypeFragment represents a fragment.
	SourceTypeFragment
)

func sourceTypeFromProto(sourceType pb.SourceType) SourceType {
	switch sourceType {
	case pb.SourceType_SOURCE_TYPE_UNSPECIFIED:
		return SourceTypeUnspecified
	case pb.SourceType_SOURCE_TYPE_ORG:
		return SourceTypeOrg
	case pb.SourceType_SOURCE_TYPE_FRAGMENT:
		return SourceTypeFragment
	default:
		return SourceTypeUnspecified
	}
}

// ResourceUsageCostsBySource contains the resource usage costs of a source type.
type ResourceUsageCostsBySource struct {
	SourceType         SourceType
	ResourceUsageCosts *ResourceUsageCosts
	TierName           string
}

func resourceUsageCostsBySourceFromProto(costs *pb.ResourceUsageCostsBySource) *ResourceUsageCostsBySource {
	return &ResourceUsageCostsBySource{
		SourceType:         sourceTypeFromProto(costs.SourceType),
		ResourceUsageCosts: resourceUsageCostsFromProto(costs.ResourceUsageCosts),
		TierName:           costs.TierName,
	}
}

// GetCurrentMonthUsageResponse contains the current month usage information.
type GetCurrentMonthUsageResponse struct {
	StartDate                  *timestamppb.Timestamp
	EndDate                    *timestamppb.Timestamp
	ResourceUsageCostsBySource []*ResourceUsageCostsBySource
	Subtotal                   float64
}

func getCurrentMonthUsageResponseFromProto(response *pb.GetCurrentMonthUsageResponse) *GetCurrentMonthUsageResponse {
	var costs []*ResourceUsageCostsBySource
	for _, cost := range response.ResourceUsageCostsBySource {
		costs = append(costs, resourceUsageCostsBySourceFromProto(cost))
	}
	return &GetCurrentMonthUsageResponse{
		StartDate:                  response.StartDate,
		EndDate:                    response.EndDate,
		ResourceUsageCostsBySource: costs,
		Subtotal:                   response.Subtotal,
	}
}

// PaymentMethodType is the type of payment method.
type PaymentMethodType int32

const (
	// PaymentMethodTypeUnspecified represents an unspecified payment method.
	PaymentMethodTypeUnspecified PaymentMethodType = iota
	// PaymentMethodtypeCard represents a payment by card.
	PaymentMethodtypeCard
)

func paymentMethodTypeFromProto(methodType pb.PaymentMethodType) PaymentMethodType {
	switch methodType {
	case pb.PaymentMethodType_PAYMENT_METHOD_TYPE_UNSPECIFIED:
		return PaymentMethodTypeUnspecified
	case pb.PaymentMethodType_PAYMENT_METHOD_TYPE_CARD:
		return PaymentMethodtypeCard
	default:
		return PaymentMethodTypeUnspecified
	}
}

// PaymentMethodCard holds the information of a card used for payment.
type PaymentMethodCard struct {
	Brand          string
	LastFourDigits string
}

func paymentMethodCardFromProto(card *pb.PaymentMethodCard) *PaymentMethodCard {
	return &PaymentMethodCard{
		Brand:          card.Brand,
		LastFourDigits: card.LastFourDigits,
	}
}

// GetOrgBillingInformationResponse contains the information of an organization's billing information.
type GetOrgBillingInformationResponse struct {
	Type         PaymentMethodType
	BillingEmail string
	// defined if type is PaymentMethodTypeCard
	Method *PaymentMethodCard
	// only return for billing dashboard admin users
	BillingTier *string
}

func getOrgBillingInformationResponseFromProto(resp *pb.GetOrgBillingInformationResponse) *GetOrgBillingInformationResponse {
	return &GetOrgBillingInformationResponse{
		Type:         paymentMethodTypeFromProto(resp.Type),
		BillingEmail: resp.BillingEmail,
		Method:       paymentMethodCardFromProto(resp.Method),
		BillingTier:  resp.BillingTier,
	}
}

// InvoiceSummary holds the information of an invoice summary.
type InvoiceSummary struct {
	ID            string
	InvoiceDate   *timestamppb.Timestamp
	InvoiceAmount float64
	Status        string
	DueDate       *timestamppb.Timestamp
	PaidDate      *timestamppb.Timestamp
}

func invoiceSummaryFromProto(summary *pb.InvoiceSummary) *InvoiceSummary {
	return &InvoiceSummary{
		ID:            summary.Id,
		InvoiceDate:   summary.InvoiceDate,
		InvoiceAmount: summary.InvoiceAmount,
		Status:        summary.Status,
		DueDate:       summary.DueDate,
		PaidDate:      summary.PaidDate,
	}
}

type invoiceStream struct {
	client       *BillingClient
	streamCancel context.CancelFunc
	streamMu     sync.Mutex

	activeBackgroundWorkers sync.WaitGroup
}

func (s *invoiceStream) startStream(ctx context.Context, id, orgID string, ch chan []byte) error {
	s.streamMu.Lock()
	defer s.streamMu.Unlock()

	if ctx.Err() != nil {
		return ctx.Err()
	}

	ctx, cancel := context.WithCancel(ctx)
	s.streamCancel = cancel

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// This call won't return any errors it had until the client tries to receive.
	//nolint:errcheck
	stream, _ := s.client.client.GetInvoicePdf(ctx, &pb.GetInvoicePdfRequest{
		Id:    id,
		OrgId: orgID,
	})
	_, err := stream.Recv()
	if err != nil {
		s.client.logger.CError(ctx, err)
		return err
	}

	// Create a background go routine to receive from the server stream.
	// We rely on calling the Done function here rather than in close stream
	// since managed go calls that function when the routine exits.
	s.activeBackgroundWorkers.Add(1)
	utils.ManagedGo(func() {
		s.receiveFromStream(ctx, stream, ch)
	},
		s.activeBackgroundWorkers.Done)
	return nil
}

func (s *invoiceStream) receiveFromStream(ctx context.Context, stream pb.BillingService_GetInvoicePdfClient, ch chan []byte) {
	defer s.streamCancel()

	// repeatedly receive from the stream
	for {
		select {
		case <-ctx.Done():
			s.client.logger.Debug(ctx.Err())
			return
		default:
		}
		streamResp, err := stream.Recv()
		if err != nil {
			// only debug log the context canceled error
			s.client.logger.Debug(err)
			return
		}
		// If there is a response, send to the channel.
		var pdf []byte
		pdf = append(pdf, streamResp.Chunk...)
		ch <- pdf
	}
}

// BillingClient is a gRPC client for method calls to the Billing API.
type BillingClient struct {
	client pb.BillingServiceClient
	logger logging.Logger

	mu sync.Mutex
}

// NewBillingClient constructs a new BillingClient using the connection passed in by the Viam client.
func NewBillingClient(conn rpc.ClientConn, logger logging.Logger) *BillingClient {
	return &BillingClient{client: pb.NewBillingServiceClient(conn), logger: logger}
}

// GetCurrentMonthUsage gets the data usage information for the current month for an organization.
func (c *BillingClient) GetCurrentMonthUsage(ctx context.Context, orgID string) (*GetCurrentMonthUsageResponse, error) {
	resp, err := c.client.GetCurrentMonthUsage(ctx, &pb.GetCurrentMonthUsageRequest{
		OrgId: orgID,
	})
	if err != nil {
		return nil, err
	}
	return getCurrentMonthUsageResponseFromProto(resp), nil
}

// GetOrgBillingInformation gets the billing information of an organization.
func (c *BillingClient) GetOrgBillingInformation(ctx context.Context, orgID string) (*GetOrgBillingInformationResponse, error) {
	resp, err := c.client.GetOrgBillingInformation(ctx, &pb.GetOrgBillingInformationRequest{
		OrgId: orgID,
	})
	if err != nil {
		return nil, err
	}
	return getOrgBillingInformationResponseFromProto(resp), nil
}

// GetInvoicesSummary returns the outstanding balance and the invoice summaries of an organization.
func (c *BillingClient) GetInvoicesSummary(ctx context.Context, orgID string) (float64, []*InvoiceSummary, error) {
	resp, err := c.client.GetInvoicesSummary(ctx, &pb.GetInvoicesSummaryRequest{
		OrgId: orgID,
	})
	if err != nil {
		return 0, nil, err
	}
	var invoices []*InvoiceSummary
	for _, invoice := range resp.Invoices {
		invoices = append(invoices, invoiceSummaryFromProto(invoice))
	}
	return resp.OutstandingBalance, invoices, nil
}

// GetInvoicePDF gets the invoice PDF data.
func (c *BillingClient) GetInvoicePDF(ctx context.Context, id, orgID string, ch chan []byte) error {
	stream := &invoiceStream{client: c}

	err := stream.startStream(ctx, id, orgID, ch)
	if err != nil {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	return nil
}

// SendPaymentRequiredEmail sends an email about payment requirement.
func (c *BillingClient) SendPaymentRequiredEmail(ctx context.Context, customerOrgID, billingOwnerOrgID string) error {
	_, err := c.client.SendPaymentRequiredEmail(ctx, &pb.SendPaymentRequiredEmailRequest{
		CustomerOrgId:     customerOrgID,
		BillingOwnerOrgId: billingOwnerOrgID,
	})
	return err
}
