package stores

type promoListCacheKey struct {
	Static string `json:"static"`
}

type promoDetailCacheKey struct {
	PromoID uint `json:"promo_id"`
}

type promoDetailCacheValue struct {
	Promo    *PromoModel
	Variants []PromoVariantModel
}

func (s *Store) getCachedPromoList(key string) ([]PromoModel, bool) {
	value, ok := s.cacheGet(key)
	if !ok {
		return nil, false
	}
	typed, ok := value.([]PromoModel)
	if !ok {
		return nil, false
	}
	return typed, true
}

func (s *Store) getCachedPromoDetail(key string) (*PromoModel, []PromoVariantModel, bool) {
	value, ok := s.cacheGet(key)
	if !ok {
		return nil, nil, false
	}
	typed, ok := value.(promoDetailCacheValue)
	if !ok || typed.Promo == nil {
		return nil, nil, false
	}
	return typed.Promo, typed.Variants, true
}

func promoListKey() string {
	return cacheKey("promos:list", promoListCacheKey{Static: "v1"})
}

func promoDetailKey(promoID uint) string {
	return cacheKey("promos:detail", promoDetailCacheKey{PromoID: promoID})
}
