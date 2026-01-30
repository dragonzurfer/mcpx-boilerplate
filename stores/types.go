package stores

const (
	AccessLevelPublic = "PUBLIC"
	AccessLevelTrial  = "TRIAL"
	AccessLevelPaid   = "PAID"
)

const (
	PostStatusDraft     = "DRAFT"
	PostStatusScheduled = "SCHEDULED"
	PostStatusPublished = "PUBLISHED"
	PostStatusArchived  = "ARCHIVED"
)

const (
	CourseStatusDraft     = "DRAFT"
	CourseStatusScheduled = "SCHEDULED"
	CourseStatusPublished = "PUBLISHED"
	CourseStatusArchived  = "ARCHIVED"
)

const (
	UserRoleUser      = "USER"
	UserRoleAdmin     = "ADMIN"
	UserRoleSuperAdmin = "SUPER_ADMIN"
)

const (
	UserStatusActive    = "ACTIVE"
	UserStatusSuspended = "SUSPENDED"
)

const (
	EntitlementStatusActive  = "ACTIVE"
	EntitlementStatusExpired = "EXPIRED"
)

const (
	PaymentStatusCreated = "CREATED"
	PaymentStatusPaid    = "PAID"
	PaymentStatusFailed  = "FAILED"
)

const (
	PromoStatusActive = "ACTIVE"
	PromoStatusPaused = "PAUSED"
)

const (
	PromoSlotInline          = "INLINE"
	PromoSlotBottomCard      = "BOTTOM_CARD"
	PromoSlotModalOnComplete = "MODAL_ON_COMPLETE"
	PromoSlotPaywallCard     = "PAYWALL_CARD"
)

const (
	FunnelStageNew        = "NEW"
	FunnelStageCasual     = "CASUAL"
	FunnelStageEngaged    = "ENGAGED"
	FunnelStageHot        = "HOT"
	FunnelStagePaidActive = "PAID_ACTIVE"
	FunnelStagePaidExpired = "PAID_EXPIRED"
	FunnelStageDormant    = "DORMANT"
)
