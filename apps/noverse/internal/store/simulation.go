package store

import (
	"os"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
)

type fileSimulationStore struct{ io *IO }

func newFileSimulationStore(io *IO) *fileSimulationStore { return &fileSimulationStore{io: io} }

func (s *fileSimulationStore) Load() (*domain.SimulationProfile, error) {
	var profile domain.SimulationProfile
	if err := s.io.ReadJSON("meta/simulation_profile.json", &profile); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if err := domain.ValidateSimulationProfile(&profile); err != nil {
		return nil, err
	}
	return &profile, nil
}

func (s *fileSimulationStore) Save(profile domain.SimulationProfile) error {
	if profile.Version == "" {
		profile.Version = domain.SimulationProfileVersion
	}
	if err := domain.ValidateSimulationProfile(&profile); err != nil {
		return err
	}
	return s.io.WriteJSON("meta/simulation_profile.json", profile)
}
