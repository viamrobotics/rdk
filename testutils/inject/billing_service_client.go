package inject

import (
	"context"

	"braces.dev/errtrace"
	billingpb "go.viam.com/api/app/v1"
	"google.golang.org/grpc"
)

// BillingServiceClient represents a fake instance of a billing service client.
type BillingServiceClient struct {
	billingpb.BillingServiceClient
	GetCurrentMonthUsageFunc func(ctx context.Context, in *billingpb.GetCurrentMonthUsageRequest,
		opts ...grpc.CallOption) (*billingpb.GetCurrentMonthUsageResponse, error)
	GetOrgBillingInformationFunc func(ctx context.Context, in *billingpb.GetOrgBillingInformationRequest,
		opts ...grpc.CallOption) (*billingpb.GetOrgBillingInformationResponse, error)
	GetInvoicesSummaryFunc func(ctx context.Context, in *billingpb.GetInvoicesSummaryRequest,
		opts ...grpc.CallOption) (*billingpb.GetInvoicesSummaryResponse, error)
	GetInvoicePdfFunc func(ctx context.Context, in *billingpb.GetInvoicePdfRequest,
		opts ...grpc.CallOption) (billingpb.BillingService_GetInvoicePdfClient, error)
	SendPaymentRequiredEmailFunc func(ctx context.Context, in *billingpb.SendPaymentRequiredEmailRequest,
		opts ...grpc.CallOption) (*billingpb.SendPaymentRequiredEmailResponse, error)
}

// GetCurrentMonthUsage calls the injected GetCurrentMonthUsageFunc or the real version.
func (bsc *BillingServiceClient) GetCurrentMonthUsage(ctx context.Context, in *billingpb.GetCurrentMonthUsageRequest,
	opts ...grpc.CallOption,
) (*billingpb.GetCurrentMonthUsageResponse, error) {
	if bsc.GetCurrentMonthUsageFunc == nil {
		return errtrace.Wrap2(bsc.BillingServiceClient.GetCurrentMonthUsage(ctx, in, opts...))
	}
	return errtrace.Wrap2(bsc.GetCurrentMonthUsageFunc(ctx, in, opts...))
}

// GetOrgBillingInformation calls the injected GetOrgBillingInformationFunc or the real version.
func (bsc *BillingServiceClient) GetOrgBillingInformation(ctx context.Context, in *billingpb.GetOrgBillingInformationRequest,
	opts ...grpc.CallOption,
) (*billingpb.GetOrgBillingInformationResponse, error) {
	if bsc.GetOrgBillingInformationFunc == nil {
		return errtrace.Wrap2(bsc.BillingServiceClient.GetOrgBillingInformation(ctx, in, opts...))
	}
	return errtrace.Wrap2(bsc.GetOrgBillingInformationFunc(ctx, in, opts...))
}

// GetInvoicesSummary calls the injected GetInvoicesSummaryFunc or the real version.
func (bsc *BillingServiceClient) GetInvoicesSummary(ctx context.Context, in *billingpb.GetInvoicesSummaryRequest,
	opts ...grpc.CallOption,
) (*billingpb.GetInvoicesSummaryResponse, error) {
	if bsc.GetInvoicesSummaryFunc == nil {
		return errtrace.Wrap2(bsc.BillingServiceClient.GetInvoicesSummary(ctx, in, opts...))
	}
	return errtrace.Wrap2(bsc.GetInvoicesSummaryFunc(ctx, in, opts...))
}

// GetInvoicePdf calls the injected GetInvoicePdfFunc or the real version.
func (bsc *BillingServiceClient) GetInvoicePdf(ctx context.Context, in *billingpb.GetInvoicePdfRequest,
	opts ...grpc.CallOption,
) (billingpb.BillingService_GetInvoicePdfClient, error) {
	if bsc.GetInvoicePdfFunc == nil {
		return errtrace.Wrap2(bsc.BillingServiceClient.GetInvoicePdf(ctx, in, opts...))
	}
	return errtrace.Wrap2(bsc.GetInvoicePdfFunc(ctx, in, opts...))
}

// BillingServiceGetInvoicePdfClient represents a fake instance of a proto BillingService_GetInvoicePdfClient.
type BillingServiceGetInvoicePdfClient struct {
	billingpb.BillingService_GetInvoicePdfClient
	RecvFunc func() (*billingpb.GetInvoicePdfResponse, error)
}

// Recv calls the injected RecvFunc or the real version.
func (c *BillingServiceGetInvoicePdfClient) Recv() (*billingpb.GetInvoicePdfResponse, error) {
	if c.RecvFunc == nil {
		return errtrace.Wrap2(c.BillingService_GetInvoicePdfClient.Recv())
	}
	return errtrace.Wrap2(c.RecvFunc())
}

// SendPaymentRequiredEmail calls the injected SendPaymentRequiredEmailFunc or the real version.
func (bsc *BillingServiceClient) SendPaymentRequiredEmail(ctx context.Context, in *billingpb.SendPaymentRequiredEmailRequest,
	opts ...grpc.CallOption,
) (*billingpb.SendPaymentRequiredEmailResponse, error) {
	if bsc.SendPaymentRequiredEmailFunc == nil {
		return errtrace.Wrap2(bsc.BillingServiceClient.SendPaymentRequiredEmail(ctx, in, opts...))
	}
	return errtrace.Wrap2(bsc.SendPaymentRequiredEmailFunc(ctx, in, opts...))
}
