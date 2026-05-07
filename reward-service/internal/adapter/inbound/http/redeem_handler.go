package http

import (
	"net/http"

	httpport "reward-service/internal/interface/http"
	"reward-service/internal/model"

	"github.com/gin-gonic/gin"
)

type RedeemHandler struct {
	service httpport.RewardRedeemHTTPPort
}

func NewRedeemHandler(s httpport.RewardRedeemHTTPPort) *RedeemHandler {
	return &RedeemHandler{
		service: s,
	}
}

type redeemRequest struct {
	RedeemID string `json:"redeem_id" binding:"required"`
	UserID   string `json:"user_id" binding:"required"`
	Point    int64  `json:"point" binding:"required"`
}

func (h *RedeemHandler) Redeem(c *gin.Context) {
	var req redeemRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if req.Point <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "point must be greater than zero"})
		return
	}

	err := h.service.RequestRedeem(
		c.Request.Context(),
		model.Redeem{
			RedeemID: req.RedeemID,
			UserID:   req.UserID,
			Point:    req.Point,
		},
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "redeem failed"})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{"status": "redeem_requested"})
}

//UI
// ↓
//HTTP Handler
// ↓
//RewardService (implements HTTP port)
// ↓
//RewardRedeemPort (interface)
// ↓
//RedeemRequestProducer (Redpanda adapter)
// ↓
//Kafka
