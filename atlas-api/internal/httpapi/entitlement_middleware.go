package httpapi

import (
	"context"
	"net/http"

	"github.com/atlas/atlas-api/internal/entitlement"
	"github.com/atlas/atlas-api/internal/httpapi/generated"
	"github.com/atlas/atlas-api/internal/httpapi/middleware"
	strictnethttp "github.com/oapi-codegen/runtime/strictmiddleware/nethttp"
	"go.uber.org/zap"
)

func (s *Server) strictEntitlementMiddleware() generated.StrictMiddlewareFunc {
	return func(
		next strictnethttp.StrictHTTPHandlerFunc,
		operationID string,
	) strictnethttp.StrictHTTPHandlerFunc {
		return func(ctx context.Context, w http.ResponseWriter, r *http.Request, request interface{}) (interface{}, error) {
			requiredEntitlement := requiredEntitlementForOperation(operationID, request)
			if requiredEntitlement == "" {
				return next(ctx, w, r, request)
			}

			userID, ok := middleware.AuthenticatedUserID(ctx)
			if !ok {
				return entitlementUnauthorizedResponse(operationID), nil
			}

			snapshot := entitlement.NewSnapshot(nil)
			if s.entitlement != nil {
				var err error
				snapshot, err = s.entitlement.SnapshotForUser(ctx, userID)
				if err != nil {
					s.logger.Error(
						"failed loading user entitlements for operation",
						zap.Error(err),
						zap.String("operation_id", operationID),
						zap.String("user_id", userID.String()),
					)
					return entitlementInternalErrorResponse(operationID), nil
				}
			}

			if !snapshot.Has(requiredEntitlement) {
				return entitlementForbiddenResponse(operationID, requiredEntitlement), nil
			}

			return next(ctx, w, r, request)
		}
	}
}

func requiredEntitlementForOperation(operationID string, request interface{}) string {
	switch operationID {
	case "GetFoodsUpcCode":
		return entitlement.BarcodeScanEntitlement
	case "GetExerciseBiomechanicsById":
		return entitlement.BiomechanicsOverlayEntitlement
	case "PostNutritionWeeklyCheckin",
		"GetNutritionWeeklyCheckinLatest",
		"PostNutritionMealPlanGenerate",
		"GetNutritionMealPlanByWeek",
		"PutNutritionMealPlan",
		"DeleteNutritionMealPlanByWeek",
		"GetNutritionMealPlanLatest":
		return entitlement.DeepNutritionEntitlement
	case "PostConsentsGrant":
		grantRequest, ok := request.(generated.PostConsentsGrantRequestObject)
		if !ok || grantRequest.Body == nil {
			return ""
		}
		if grantRequest.Body.ConsentType == generated.ConsentTypeFormCheckUpload {
			return entitlement.FormCheckUploadEntitlement
		}
		return ""
	case "PostFormCheckUploads":
		return entitlement.FormCheckUploadEntitlement
	default:
		return ""
	}
}

func entitlementUnauthorizedResponse(operationID string) interface{} {
	switch operationID {
	case "GetFoodsUpcCode":
		return generated.GetFoodsUpcCode401JSONResponse{Message: "unauthorized"}
	case "GetExerciseBiomechanicsById":
		return generated.GetExerciseBiomechanicsById401JSONResponse{Message: "unauthorized"}
	case "PostNutritionWeeklyCheckin":
		return generated.PostNutritionWeeklyCheckin401JSONResponse{Message: "unauthorized"}
	case "GetNutritionWeeklyCheckinLatest":
		return generated.GetNutritionWeeklyCheckinLatest401JSONResponse{Message: "unauthorized"}
	case "PostNutritionMealPlanGenerate":
		return generated.PostNutritionMealPlanGenerate401JSONResponse{Message: "unauthorized"}
	case "GetNutritionMealPlanByWeek":
		return generated.GetNutritionMealPlanByWeek401JSONResponse{Message: "unauthorized"}
	case "PutNutritionMealPlan":
		return generated.PutNutritionMealPlan401JSONResponse{Message: "unauthorized"}
	case "DeleteNutritionMealPlanByWeek":
		return generated.DeleteNutritionMealPlanByWeek401JSONResponse{Message: "unauthorized"}
	case "GetNutritionMealPlanLatest":
		return generated.GetNutritionMealPlanLatest401JSONResponse{Message: "unauthorized"}
	case "PostConsentsGrant":
		return generated.PostConsentsGrant401JSONResponse{Message: "unauthorized"}
	case "PostFormCheckUploads":
		return generated.PostFormCheckUploads401JSONResponse{Message: "unauthorized"}
	default:
		return generated.ErrorResponse{Message: "unauthorized"}
	}
}

func entitlementForbiddenResponse(operationID, requiredEntitlement string) interface{} {
	message := "subscription entitlement required: " + requiredEntitlement
	switch operationID {
	case "GetFoodsUpcCode":
		return generated.GetFoodsUpcCode403JSONResponse{Message: message}
	case "GetExerciseBiomechanicsById":
		return generated.GetExerciseBiomechanicsById403JSONResponse{Message: message}
	case "PostNutritionWeeklyCheckin":
		return generated.PostNutritionWeeklyCheckin403JSONResponse{Message: message}
	case "GetNutritionWeeklyCheckinLatest":
		return generated.GetNutritionWeeklyCheckinLatest403JSONResponse{Message: message}
	case "PostNutritionMealPlanGenerate":
		return generated.PostNutritionMealPlanGenerate403JSONResponse{Message: message}
	case "GetNutritionMealPlanByWeek":
		return generated.GetNutritionMealPlanByWeek403JSONResponse{Message: message}
	case "PutNutritionMealPlan":
		return generated.PutNutritionMealPlan403JSONResponse{Message: message}
	case "DeleteNutritionMealPlanByWeek":
		return generated.DeleteNutritionMealPlanByWeek403JSONResponse{Message: message}
	case "GetNutritionMealPlanLatest":
		return generated.GetNutritionMealPlanLatest403JSONResponse{Message: message}
	case "PostConsentsGrant":
		return generated.PostConsentsGrant403JSONResponse{Message: message}
	case "PostFormCheckUploads":
		return generated.PostFormCheckUploads403JSONResponse{Message: message}
	default:
		return generated.ErrorResponse{Message: message}
	}
}

func entitlementInternalErrorResponse(operationID string) interface{} {
	switch operationID {
	case "GetFoodsUpcCode":
		return generated.GetFoodsUpcCode500JSONResponse{Message: "could not verify entitlement"}
	case "GetExerciseBiomechanicsById":
		return generated.GetExerciseBiomechanicsById500JSONResponse{Message: "could not verify entitlement"}
	case "PostNutritionWeeklyCheckin":
		return generated.PostNutritionWeeklyCheckin500JSONResponse{Message: "could not verify entitlement"}
	case "GetNutritionWeeklyCheckinLatest":
		return generated.GetNutritionWeeklyCheckinLatest500JSONResponse{Message: "could not verify entitlement"}
	case "PostNutritionMealPlanGenerate":
		return generated.PostNutritionMealPlanGenerate500JSONResponse{Message: "could not verify entitlement"}
	case "GetNutritionMealPlanByWeek":
		return generated.GetNutritionMealPlanByWeek500JSONResponse{Message: "could not verify entitlement"}
	case "PutNutritionMealPlan":
		return generated.PutNutritionMealPlan500JSONResponse{Message: "could not verify entitlement"}
	case "DeleteNutritionMealPlanByWeek":
		return generated.DeleteNutritionMealPlanByWeek500JSONResponse{Message: "could not verify entitlement"}
	case "GetNutritionMealPlanLatest":
		return generated.GetNutritionMealPlanLatest500JSONResponse{Message: "could not verify entitlement"}
	case "PostConsentsGrant":
		return generated.PostConsentsGrant500JSONResponse{Message: "could not verify entitlement"}
	case "PostFormCheckUploads":
		return generated.PostFormCheckUploads500JSONResponse{Message: "could not verify entitlement"}
	default:
		return generated.ErrorResponse{Message: "could not verify entitlement"}
	}
}
