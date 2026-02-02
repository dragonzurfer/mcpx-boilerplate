package stores

import "gorm.io/gorm"

func (s *Store) ListPromos() ([]PromoModel, error) {
	key := promoListKey()
	if cached, ok := s.getCachedPromoList(key); ok {
		return cached, nil
	}

	rows := []PromoModel{}
	if err := s.db.Find(&rows).Error; err != nil {
		return nil, err
	}
	s.cacheSet(key, rows)
	return rows, nil
}

func (s *Store) GetPromoWithVariants(promoID uint) (*PromoModel, []PromoVariantModel, error) {
	if promoID == 0 {
		return nil, nil, gorm.ErrRecordNotFound
	}
	key := promoDetailKey(promoID)
	if promo, variants, ok := s.getCachedPromoDetail(key); ok {
		return promo, variants, nil
	}

	var promo PromoModel
	if err := s.db.First(&promo, promoID).Error; err != nil {
		return nil, nil, err
	}

	var variants []PromoVariantModel
	if err := s.db.Where("promo_id = ?", promo.ID).Find(&variants).Error; err != nil {
		return nil, nil, err
	}

	s.cacheSet(key, promoDetailCacheValue{Promo: &promo, Variants: variants})
	return &promo, variants, nil
}
