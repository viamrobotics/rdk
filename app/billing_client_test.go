package app

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	pb "go.viam.com/api/app/v1"
	"go.viam.com/test"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.viam.com/rdk/testutils/inject"
)

const (
	subtotal                     = 37
	sourceType                   = SourceTypeOrg
	usageCostType                = UsageCostTypeCloudStorage
	cost                 float64 = 20
	discount             float64 = 9
	totalWithDiscount            = cost - discount
	totalWithoutDiscount float64 = cost
	paymentMethodType            = PaymentMethodtypeCard
	brand                        = "brand"
	digits                       = "1234"
	invoiceID                    = "invoice_id"
	invoiceAmount        float64 = 100.12
	statusString                 = "status"
	balance              float64 = 73.21
	billingOwnerOrgID            = "billing_owner_organization_id"
)

var (
	tier                         = "tier"
	getCurrentMonthUsageResponse = GetCurrentMonthUsageResponse{
		StartDate: &start,
		EndDate:   &end,
		ResourceUsageCostsBySource: []*ResourceUsageCostsBySource{
			{
				SourceType: sourceType,
				ResourceUsageCosts: &ResourceUsageCosts{
					UsageCosts: []*UsageCost{
						{
							ResourceType: usageCostType,
							Cost:         cost,
						},
					},
					Discount:             discount,
					TotalWithDiscount:    totalWithDiscount,
					TotalWithoutDiscount: totalWithoutDiscount,
				},
				TierName: tier,
			},
		},
		Subtotal: subtotal,
	}
	getOrgBillingInformationResponse = GetOrgBillingInformationResponse{
		Type:         paymentMethodType,
		BillingEmail: email,
		Method: &PaymentMethodCard{
			Brand:          brand,
			LastFourDigits: digits,
		},
		BillingTier: &tier,
	}
	invoiceDate    = time.Now().UTC().Round(time.Millisecond).UTC()
	dueDate        = time.Now().UTC().Round(time.Millisecond).UTC()
	paidDate       = time.Now().UTC().Round(time.Millisecond).UTC()
	invoiceSummary = InvoiceSummary{
		ID:            invoiceID,
		InvoiceDate:   &invoiceDate,
		InvoiceAmount: invoiceAmount,
		Status:        statusString,
		DueDate:       &dueDate,
		PaidDate:      &paidDate,
	}
	chunk2     = []byte("chunk1")
	chunk3     = []byte("chunk2")
	chunks     = [][]byte{byteData, chunk2, chunk3}
	chunkCount = len(chunks)
)

func sourceTypeToProto(sourceType SourceType) pb.SourceType {
	switch sourceType {
	case SourceTypeUnspecified:
		return pb.SourceType_SOURCE_TYPE_UNSPECIFIED
	case SourceTypeOrg:
		return pb.SourceType_SOURCE_TYPE_ORG
	case SourceTypeFragment:
		return pb.SourceType_SOURCE_TYPE_FRAGMENT
	default:
		return pb.SourceType_SOURCE_TYPE_UNSPECIFIED
	}
}

func usageCostTypeToProto(costType UsageCostType) pb.UsageCostType {
	switch costType {
	case UsageCostTypeUnspecified:
		return pb.UsageCostType_USAGE_COST_TYPE_UNSPECIFIED
	case UsageCostTypeDataUpload:
		//nolint:deprecated,staticcheck
		return pb.UsageCostType_USAGE_COST_TYPE_DATA_UPLOAD
	case UsageCostTypeDataEgress:
		//nolint:deprecated,staticcheck
		return pb.UsageCostType_USAGE_COST_TYPE_DATA_EGRESS
	case UsageCostTypeRemoteControl:
		return pb.UsageCostType_USAGE_COST_TYPE_REMOTE_CONTROL
	case UsageCostTypeStandardCompute:
		return pb.UsageCostType_USAGE_COST_TYPE_STANDARD_COMPUTE
	case UsageCostTypeCloudStorage:
		//nolint:deprecated,staticcheck
		return pb.UsageCostType_USAGE_COST_TYPE_CLOUD_STORAGE
	case UsageCostTypeBinaryDataCloudStorage:
		return pb.UsageCostType_USAGE_COST_TYPE_BINARY_DATA_CLOUD_STORAGE
	case UsageCostTypeOtherCloudStorage:
		//nolint:deprecated,staticcheck
		return pb.UsageCostType_USAGE_COST_TYPE_OTHER_CLOUD_STORAGE
	case UsageCostTypePerMachine:
		return pb.UsageCostType_USAGE_COST_TYPE_PER_MACHINE
	default:
		return pb.UsageCostType_USAGE_COST_TYPE_UNSPECIFIED
	}
}

func paymentMethodTypeToProto(methodType PaymentMethodType) pb.PaymentMethodType {
	switch methodType {
	case PaymentMethodTypeUnspecified:
		return pb.PaymentMethodType_PAYMENT_METHOD_TYPE_UNSPECIFIED
	case PaymentMethodtypeCard:
		return pb.PaymentMethodType_PAYMENT_METHOD_TYPE_CARD
	default:
		return pb.PaymentMethodType_PAYMENT_METHOD_TYPE_UNSPECIFIED
	}
}

func createBillingGrpcClient() *inject.BillingServiceClient {
	return &inject.BillingServiceClient{}
}

func TestBillingClient(t *testing.T) {
	grpcClient := createBillingGrpcClient()
	client := BillingClient{client: grpcClient}

	t.Run("GetCurrentMonthUsage", func(t *testing.T) {
		pbResponse := pb.GetCurrentMonthUsageResponse{
			StartDate: timestamppb.New(*getCurrentMonthUsageResponse.StartDate),
			EndDate:   timestamppb.New(*getCurrentMonthUsageResponse.EndDate),
			ResourceUsageCostsBySource: []*pb.ResourceUsageCostsBySource{
				{
					SourceType: sourceTypeToProto(sourceType),
					ResourceUsageCosts: &pb.ResourceUsageCosts{
						UsageCosts: []*pb.UsageCost{
							{
								ResourceType: usageCostTypeToProto(usageCostType),
								Cost:         cost,
							},
						},
						Discount:             discount,
						TotalWithDiscount:    totalWithDiscount,
						TotalWithoutDiscount: totalWithoutDiscount,
					},
					TierName: tier,
				},
			},
			Subtotal: getCurrentMonthUsageResponse.Subtotal,
		}
		grpcClient.GetCurrentMonthUsageFunc = func(
			ctx context.Context, in *pb.GetCurrentMonthUsageRequest, opts ...grpc.CallOption,
		) (*pb.GetCurrentMonthUsageResponse, error) {
			test.That(t, in.OrgId, test.ShouldEqual, organizationID)
			return &pbResponse, nil
		}
		resp, err := client.GetCurrentMonthUsage(context.Background(), organizationID)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldResemble, &getCurrentMonthUsageResponse)
	})

	t.Run("GetOrgBillingInformation", func(t *testing.T) {
		pbResponse := pb.GetOrgBillingInformationResponse{
			Type:         paymentMethodTypeToProto(getOrgBillingInformationResponse.Type),
			BillingEmail: getOrgBillingInformationResponse.BillingEmail,
			Method: &pb.PaymentMethodCard{
				Brand:          getOrgBillingInformationResponse.Method.Brand,
				LastFourDigits: getOrgBillingInformationResponse.Method.LastFourDigits,
			},
			BillingTier: getOrgBillingInformationResponse.BillingTier,
		}
		grpcClient.GetOrgBillingInformationFunc = func(
			ctx context.Context, in *pb.GetOrgBillingInformationRequest, opts ...grpc.CallOption,
		) (*pb.GetOrgBillingInformationResponse, error) {
			test.That(t, in.OrgId, test.ShouldEqual, organizationID)
			return &pbResponse, nil
		}
		resp, err := client.GetOrgBillingInformation(context.Background(), organizationID)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldResemble, &getOrgBillingInformationResponse)
	})

	t.Run("GetInvoicesSummary", func(t *testing.T) {
		expectedInvoices := []*InvoiceSummary{&invoiceSummary}
		grpcClient.GetInvoicesSummaryFunc = func(
			ctx context.Context, in *pb.GetInvoicesSummaryRequest, opts ...grpc.CallOption,
		) (*pb.GetInvoicesSummaryResponse, error) {
			test.That(t, in.OrgId, test.ShouldEqual, organizationID)
			return &pb.GetInvoicesSummaryResponse{
				OutstandingBalance: balance,
				Invoices: []*pb.InvoiceSummary{
					{
						Id:            invoiceSummary.ID,
						InvoiceDate:   timestamppb.New(*invoiceSummary.InvoiceDate),
						InvoiceAmount: invoiceSummary.InvoiceAmount,
						Status:        invoiceSummary.Status,
						DueDate:       timestamppb.New(*invoiceSummary.DueDate),
						PaidDate:      timestamppb.New(*invoiceSummary.PaidDate),
					},
				},
			}, nil
		}
		outstandingBalance, invoices, err := client.GetInvoicesSummary(context.Background(), organizationID)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, outstandingBalance, test.ShouldResemble, balance)
		test.That(t, invoices, test.ShouldResemble, expectedInvoices)
	})

	t.Run("GetInvoicePDF", func(t *testing.T) {
		expectedData := bytes.Join(chunks, nil)
		var count int
		mockStream := &inject.BillingServiceGetInvoicePdfClient{
			RecvFunc: func() (*pb.GetInvoicePdfResponse, error) {
				if count >= chunkCount {
					return nil, io.EOF
				}
				chunk := chunks[count]
				count++
				return &pb.GetInvoicePdfResponse{
					Chunk: chunk,
				}, nil
			},
		}
		grpcClient.GetInvoicePdfFunc = func(
			ctx context.Context, in *pb.GetInvoicePdfRequest, opts ...grpc.CallOption,
		) (pb.BillingService_GetInvoicePdfClient, error) {
			test.That(t, in.Id, test.ShouldEqual, invoiceID)
			test.That(t, in.OrgId, test.ShouldEqual, organizationID)
			return mockStream, nil
		}
		data, err := client.GetInvoicePDF(context.Background(), invoiceID, organizationID)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, data, test.ShouldResemble, expectedData)
	})

	t.Run("SendPaymentRequiredEmail", func(t *testing.T) {
		grpcClient.SendPaymentRequiredEmailFunc = func(
			ctx context.Context, in *pb.SendPaymentRequiredEmailRequest, opts ...grpc.CallOption,
		) (*pb.SendPaymentRequiredEmailResponse, error) {
			test.That(t, in.CustomerOrgId, test.ShouldEqual, organizationID)
			test.That(t, in.BillingOwnerOrgId, test.ShouldEqual, billingOwnerOrgID)
			return &pb.SendPaymentRequiredEmailResponse{}, nil
		}
		err := client.SendPaymentRequiredEmail(context.Background(), organizationID, billingOwnerOrgID)
		test.That(t, err, test.ShouldBeNil)
	})
}
