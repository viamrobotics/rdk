package app

import (
	"context"

	pb "go.viam.com/api/app/v1"
	"go.viam.com/utils/rpc"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type UsageCostType int32

const (
	UsageCostTypeUnspecified UsageCostType = iota
	UsageCostTypeDataUpload
	UsageCostTypeDataEgress
	UsageCostTypeRemoteControl
	UsageCostTypeStandardCompute
	UsageCostTypeCloudStorage
	UsageCostTypeBinaryDataCloudStorage
	UsageCostTypeOtherCloudStorage
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

type UsageCost struct {
	ResourceType UsageCostType
	Cost float64
}

func usageCostFromProto(cost *pb.UsageCost) *UsageCost {
	return &UsageCost{
		ResourceType: usageCostTypeFromProto(cost.ResourceType),
		Cost: cost.Cost,
	}
}

type ResourceUsageCosts struct {
	UsageCosts []*UsageCost
	Discount float64
	TotalWithDiscount float64
	TotalWithoutDiscount float64
}

func resourceUsageCostsFromProto(costs *pb.ResourceUsageCosts) *ResourceUsageCosts {
	var usageCosts []*UsageCost
	for _, cost := range(costs.UsageCosts) {
		usageCosts = append(usageCosts, usageCostFromProto(cost))
	}
	return &ResourceUsageCosts{
		UsageCosts: usageCosts,
		Discount: costs.Discount,
		TotalWithDiscount: costs.TotalWithDiscount,
		TotalWithoutDiscount: costs.TotalWithoutDiscount,
	}
}

type ResourceUsageCostsBySource struct {
	SourceType pb.SourceType
	ResourceUsageCosts *ResourceUsageCosts
	TierName string
}

func resourceUsageCostsBySourceFromProto(costs *pb.ResourceUsageCostsBySource) *ResourceUsageCostsBySource {
	return &ResourceUsageCostsBySource{
		SourceType: costs.SourceType,
		ResourceUsageCosts: resourceUsageCostsFromProto(costs.ResourceUsageCosts),
		TierName: costs.TierName,
	}
}

type GetCurrentMonthUsageResponse struct {
	StartDate *timestamppb.Timestamp
	EndDate *timestamppb.Timestamp
	ResourceUsageCostsBySource []*ResourceUsageCostsBySource
	Subtotal float64
}

func getCurrentMonthUsageResponseFromProto(response *pb.GetCurrentMonthUsageResponse) *GetCurrentMonthUsageResponse {
	var costs []*ResourceUsageCostsBySource
	for _, cost := range(response.ResourceUsageCostsBySource) {
		costs = append(costs, resourceUsageCostsBySourceFromProto(cost))
	}
	return &GetCurrentMonthUsageResponse{
		StartDate: response.StartDate,
		EndDate: response.EndDate,
		ResourceUsageCostsBySource: costs,
		Subtotal: response.Subtotal,
	}
}

type PaymentMethodType int32

const (
	PaymentMethodTypeUnspecified PaymentMethodType = iota
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

type PaymentMethodCard struct {
	Brand string
	LastFourDigits string
}

func paymentMethodCardFromProto(card *pb.PaymentMethodCard) *PaymentMethodCard {
	return &PaymentMethodCard{
		Brand: card.Brand,
		LastFourDigits: card.LastFourDigits,
	}
}

type GetOrgBillingInformationResponse struct {
	Type PaymentMethodType
	BillingEmail string
	// defined if type is PaymentMethodTypeCard
	Method *PaymentMethodCard
	// only return for billing dashboard admin users
	BillingTier *string
}

func getOrgBillingInformationResponseFromProto(resp *pb.GetOrgBillingInformationResponse) *GetOrgBillingInformationResponse {
	return &GetOrgBillingInformationResponse{
		Type: paymentMethodTypeFromProto(resp.Type),
		BillingEmail: resp.BillingEmail,
		Method: paymentMethodCardFromProto(resp.Method),
		BillingTier: resp.BillingTier,
	}
}

type InvoiceSummary struct {
	ID string
	InvoiceDate *timestamppb.Timestamp
	InvoiceAmount float64
	Status string
	DueDate *timestamppb.Timestamp
	PaidDate *timestamppb.Timestamp
}

func invoiceSummaryFromProto(summary *pb.InvoiceSummary) *InvoiceSummary {
	return &InvoiceSummary{
		ID: summary.Id,
		InvoiceDate: summary.InvoiceDate,
		InvoiceAmount: summary.InvoiceAmount,
		Status: summary.Status,
		DueDate: summary.DueDate,
		PaidDate: summary.PaidDate,
	}
}

type BillingClient struct {
	client pb.BillingServiceClient
}

func NewBillingClient(conn rpc.ClientConn) *BillingClient {
	return &BillingClient{client: pb.NewBillingServiceClient(conn)}
}

func (c *BillingClient) GetCurrentMonthUsage(ctx context.Context, orgID string) (*GetCurrentMonthUsageResponse, error) {
	resp, err := c.client.GetCurrentMonthUsage(ctx, &pb.GetCurrentMonthUsageRequest{
		OrgId: orgID,
	})
	if err != nil {
		return nil, err
	}
	return getCurrentMonthUsageResponseFromProto(resp), nil
}

func (c *BillingClient) GetOrgBillingInformation(ctx context.Context, orgID string) (*GetOrgBillingInformationResponse, error) {
	resp, err := c.client.GetOrgBillingInformation(ctx, &pb.GetOrgBillingInformationRequest{
		OrgId: orgID,
	})
	if err != nil {
		return nil, err
	}
	return getOrgBillingInformationResponseFromProto(resp), nil
}

func (c *BillingClient) GetInvoicesSummary(ctx context.Context, orgID string) (float64, []*InvoiceSummary, error) {
	resp, err := c.client.GetInvoicesSummary(ctx, &pb.GetInvoicesSummaryRequest{
		OrgId: orgID,
	})
	if err != nil {
		return 0, nil, err
	}
	var invoices []*InvoiceSummary
	for _, invoice := range(resp.Invoices) {
		invoices = append(invoices, invoiceSummaryFromProto(invoice))
	}
	return resp.OutstandingBalance, invoices, nil
}

func (c *BillingClient) GetInvoicePdf(ctx context.Context, id, orgID string) () {}

func (c *BillingClient) SendPaymentRequiredEmail(ctx context.Context, customerOrgID, billingOwnerOrgID string) error {
	_, err := c.client.SendPaymentRequiredEmail(ctx, &pb.SendPaymentRequiredEmailRequest{
		CustomerOrgId: customerOrgID,
		BillingOwnerOrgId: billingOwnerOrgID,
	})
	return err
}
