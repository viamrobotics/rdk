package inject

import (
	"context"

	pb "go.viam.com/api/app/v1"
	"google.golang.org/grpc"
)

// BillingServiceClient represents a fake instance of a billing service client.
type BillingServiceClient struct {
	pb.BillingServiceClient
	GetCurrentMonthUsageFunc func(ctx context.Context, in *pb.GetCurrentMonthUsageRequest,
		opts ...grpc.CallOption) (*pb.GetCurrentMonthUsageResponse, error)
	GetOrgBillingInformationFunc func(ctx context.Context, in *pb.GetOrgBillingInformationRequest,
		opts ...grpc.CallOption) (*pb.GetOrgBillingInformationResponse, error)
	GetInvoicesSummaryFunc func(ctx context.Context, in *pb.GetInvoicesSummaryRequest,
		opts ...grpc.CallOption) (*pb.GetInvoicesSummaryResponse, error)
	GetInvoicePdfFunc func(ctx context.Context, in *pb.GetInvoicePdfRequest,
		opts ...grpc.CallOption) (pb.BillingService_GetInvoicePdfClient, error)
	SendPaymentRequiredEmailFunc func(ctx context.Context, in *pb.SendPaymentRequiredEmailRequest,
		opts ...grpc.CallOption) (*pb.SendPaymentRequiredEmailResponse, error)
}

// GetCurrentMonthUsage calls the injected GetCurrentMonthUsageFunc or the real version.
func (bsc *BillingServiceClient) GetCurrentMonthUsage(ctx context.Context, in *pb.GetCurrentMonthUsageRequest,
	opts ...grpc.CallOption,
) (*pb.GetCurrentMonthUsageResponse, error) {
	if bsc.GetCurrentMonthUsageFunc == nil {
		return bsc.BillingServiceClient.GetCurrentMonthUsage(ctx, in, opts...)
	}
	return bsc.GetCurrentMonthUsageFunc(ctx, in, opts...)
}

// GetOrgBillingInformation calls the injected GetOrgBillingInformationFunc or the real version.
func (bsc *BillingServiceClient) GetOrgBillingInformation(ctx context.Context, in *pb.GetOrgBillingInformationRequest,
	opts ...grpc.CallOption,
) (*pb.GetOrgBillingInformationResponse, error) {
	if bsc.GetOrgBillingInformationFunc == nil {
		return bsc.BillingServiceClient.GetOrgBillingInformation(ctx, in, opts...)
	}
	return bsc.GetOrgBillingInformationFunc(ctx, in, opts...)
}

// GetInvoicesSummary calls the injected GetInvoicesSummaryFunc or the real version.
func (bsc *BillingServiceClient) GetInvoicesSummary(ctx context.Context, in *pb.GetInvoicesSummaryRequest,
	opts ...grpc.CallOption,
) (*pb.GetInvoicesSummaryResponse, error) {
	if bsc.GetInvoicesSummaryFunc == nil {
		return bsc.BillingServiceClient.GetInvoicesSummary(ctx, in, opts...)
	}
	return bsc.GetInvoicesSummaryFunc(ctx, in, opts...)
}

// GetInvoicePdf calls the injected GetInvoicePdfFunc or the real version.
func (bsc *BillingServiceClient) GetInvoicePdf(ctx context.Context, in *pb.GetInvoicePdfRequest,
	opts ...grpc.CallOption,
) (pb.BillingService_GetInvoicePdfClient, error) {
	if bsc.GetInvoicePdfFunc == nil {
		return bsc.BillingServiceClient.GetInvoicePdf(ctx, in, opts...)
	}
	return bsc.GetInvoicePdfFunc(ctx, in, opts...)
}

// SendPaymentRequiredEmail calls the injected SendPaymentRequiredEmailFunc or the real version.
func (bsc *BillingServiceClient) SendPaymentRequiredEmail(ctx context.Context, in *pb.SendPaymentRequiredEmailRequest,
	opts ...grpc.CallOption,
) (*pb.SendPaymentRequiredEmailResponse, error) {
	if bsc.SendPaymentRequiredEmailFunc == nil {
		return bsc.BillingServiceClient.SendPaymentRequiredEmail(ctx, in, opts...)
	}
	return bsc.SendPaymentRequiredEmailFunc(ctx, in, opts...)
}
