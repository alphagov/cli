package v7action_test

import (
	. "code.cloudfoundry.org/cli/actor/v7action"
	"code.cloudfoundry.org/cli/actor/v7action/v7actionfakes"
	"code.cloudfoundry.org/cli/api/cloudcontroller/ccv3"
	"errors"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Domain Actions", func() {
	var (
		actor                     *Actor
		fakeCloudControllerClient *v7actionfakes.FakeCloudControllerClient
	)

	BeforeEach(func() {
		actor, fakeCloudControllerClient, _, _, _ = NewTestActor()
	})

	Describe("create shared domain", func() {
		It("delegates to the cloud controller client", func() {
			fakeCloudControllerClient.CreateDomainReturns(ccv3.Domain{}, ccv3.Warnings{"create-warning-1", "create-warning-2"}, errors.New("create-error"))

			warnings, executeErr := actor.CreateSharedDomain("the-domain-name", true)
			Expect(executeErr).To(MatchError("create-error"))
			Expect(warnings).To(ConsistOf("create-warning-1", "create-warning-2"))

			Expect(fakeCloudControllerClient.CreateDomainCallCount()).To(Equal(1))
			passedDomain := fakeCloudControllerClient.CreateDomainArgsForCall(0)

			Expect(passedDomain).To(Equal(
				ccv3.Domain{
					Name:     "the-domain-name",
					Internal: true,
				},
			))
		})
	})
})
