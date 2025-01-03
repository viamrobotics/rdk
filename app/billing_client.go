package app

import (
	"context"
	"errors"
	"io"
	"time"

	pb "go.viam.com/api/app/v1"
	"go.viam.com/utils/rpc"
)

// UsageCostType specifies the type of usage cost.
type UsageCostType int

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

// UsageCost contains the cost and cost type.
type UsageCost struct {
	ResourceType UsageCostType
	Cost         float64
}

// ResourceUsageCosts holds the usage costs with discount information.
type ResourceUsageCosts struct {
	UsageCosts           []*UsageCost
	Discount             float64
	TotalWithDiscount    float64
	TotalWithoutDiscount float64
}

// SourceType is the type of source from which a cost is coming from.
type SourceType int

const (
	// SourceTypeUnspecified represents an unspecified source type.
	SourceTypeUnspecified SourceType = iota
	// SourceTypeOrg represents an organization.
	SourceTypeOrg
	// SourceTypeFragment represents a fragment.
	SourceTypeFragment
)

// ResourceUsageCostsBySource contains the resource usage costs of a source.
type ResourceUsageCostsBySource struct {
	SourceType         SourceType
	ResourceUsageCosts *ResourceUsageCosts
	TierName           string
}

// GetCurrentMonthUsageResponse contains the current month usage information.
type GetCurrentMonthUsageResponse struct {
	StartDate                  *time.Time
	EndDate                    *time.Time
	ResourceUsageCostsBySource []*ResourceUsageCostsBySource
	Subtotal                   float64
}

// PaymentMethodType is the type of payment method.
type PaymentMethodType int

const (
	// PaymentMethodTypeUnspecified represents an unspecified payment method.
	PaymentMethodTypeUnspecified PaymentMethodType = iota
	// PaymentMethodtypeCard represents a payment by card.
	PaymentMethodtypeCard
)

// PaymentMethodCard holds the information of a card used for payment.
type PaymentMethodCard struct {
	Brand          string
	LastFourDigits string
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

// InvoiceSummary holds the information of an invoice summary.
type InvoiceSummary struct {
	ID            string
	InvoiceDate   *time.Time
	InvoiceAmount float64
	Status        string
	DueDate       *time.Time
	PaidDate      *time.Time
}

// BillingClient is a gRPC client for method calls to the Billing API.
type BillingClient struct {
	client pb.BillingServiceClient
}

func newBillingClient(conn rpc.ClientConn) *BillingClient {
	return &BillingClient{client: pb.NewBillingServiceClient(conn)}
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

// GetInvoicePDF returns raw byte slices representing the invoice PDF data.
func (c *BillingClient) GetInvoicePDF(ctx context.Context, id, orgID string) ([]byte, error) {
	stream, err := c.client.GetInvoicePdf(ctx, &pb.GetInvoicePdfRequest{
		Id:    id,
		OrgId: orgID,
	})
	if err != nil {
		return nil, err
	}

	var data []byte
	for {
		resp, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return data, err
		}
		data = append(data, resp.Chunk...)
	}

	return data, nil
}

// SendPaymentRequiredEmail sends an email about payment requirement.
func (c *BillingClient) SendPaymentRequiredEmail(ctx context.Context, customerOrgID, billingOwnerOrgID string) error {
	_, err := c.client.SendPaymentRequiredEmail(ctx, &pb.SendPaymentRequiredEmailRequest{
		CustomerOrgId:     customerOrgID,
		BillingOwnerOrgId: billingOwnerOrgID,
	})
	return err
}

func usageCostTypeFromProto(costType pb.UsageCostType) UsageCostType {
	//nolint:exhaustive
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

func usageCostFromProto(cost *pb.UsageCost) *UsageCost {
	if cost == nil {
		return nil
	}
	return &UsageCost{
		ResourceType: usageCostTypeFromProto(cost.ResourceType),
		Cost:         cost.Cost,
	}
}

func resourceUsageCostsFromProto(costs *pb.ResourceUsageCosts) *ResourceUsageCosts {
	if costs == nil {
		return nil
	}
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

func resourceUsageCostsBySourceFromProto(costs *pb.ResourceUsageCostsBySource) *ResourceUsageCostsBySource {
	if costs == nil {
		return nil
	}
	var usageCosts *ResourceUsageCosts
	if costs.ResourceUsageCosts != nil {
		usageCosts = resourceUsageCostsFromProto(costs.ResourceUsageCosts)
	}
	return &ResourceUsageCostsBySource{
		SourceType:         sourceTypeFromProto(costs.SourceType),
		ResourceUsageCosts: usageCosts,
		TierName:           costs.TierName,
	}
}

func getCurrentMonthUsageResponseFromProto(response *pb.GetCurrentMonthUsageResponse) *GetCurrentMonthUsageResponse {
	if response == nil {
		return nil
	}
	var startDate, endDate *time.Time
	if response.StartDate != nil {
		t := response.StartDate.AsTime()
		startDate = &t
	}
	if response.EndDate != nil {
		t := response.EndDate.AsTime()
		endDate = &t
	}
	var costs []*ResourceUsageCostsBySource
	for _, cost := range response.ResourceUsageCostsBySource {
		costs = append(costs, resourceUsageCostsBySourceFromProto(cost))
	}
	return &GetCurrentMonthUsageResponse{
		StartDate:                  startDate,
		EndDate:                    endDate,
		ResourceUsageCostsBySource: costs,
		Subtotal:                   response.Subtotal,
	}
}

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

func paymentMethodCardFromProto(card *pb.PaymentMethodCard) *PaymentMethodCard {
	if card == nil {
		return nil
	}
	return &PaymentMethodCard{
		Brand:          card.Brand,
		LastFourDigits: card.LastFourDigits,
	}
}

func getOrgBillingInformationResponseFromProto(resp *pb.GetOrgBillingInformationResponse) *GetOrgBillingInformationResponse {
	if resp == nil {
		return nil
	}
	return &GetOrgBillingInformationResponse{
		Type:         paymentMethodTypeFromProto(resp.Type),
		BillingEmail: resp.BillingEmail,
		Method:       paymentMethodCardFromProto(resp.Method),
		BillingTier:  resp.BillingTier,
	}
}

func invoiceSummaryFromProto(summary *pb.InvoiceSummary) *InvoiceSummary {
	if summary == nil {
		return nil
	}
	var invoiceDate, dueDate, paidDate *time.Time
	if summary.InvoiceDate != nil {
		t := summary.InvoiceDate.AsTime()
		invoiceDate = &t
	}
	if summary.DueDate != nil {
		t := summary.DueDate.AsTime()
		dueDate = &t
	}
	if summary.PaidDate != nil {
		t := summary.PaidDate.AsTime()
		paidDate = &t
	}
	return &InvoiceSummary{
		ID:            summary.Id,
		InvoiceDate:   invoiceDate,
		InvoiceAmount: summary.InvoiceAmount,
		Status:        summary.Status,
		DueDate:       dueDate,
		PaidDate:      paidDate,
	}
}
