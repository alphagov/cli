package v7pushaction_test

import (
	"errors"
	"time"

	"code.cloudfoundry.org/cli/actor/actionerror"
	"code.cloudfoundry.org/cli/actor/v7action"
	"code.cloudfoundry.org/cli/util/manifestparser"

	. "code.cloudfoundry.org/cli/actor/v7pushaction"
	"code.cloudfoundry.org/cli/actor/v7pushaction/v7pushactionfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	log "github.com/sirupsen/logrus"
)

func PrepareSpaceStreamsDrainedAndClosed(
	pushPlansStream <-chan []PushPlan,
	eventStream <-chan Event,
	warningsStream <-chan Warnings,
	errorStream <-chan error,
) bool {
	var configStreamClosed, eventStreamClosed, warningsStreamClosed, errorStreamClosed bool
	for {
		select {
		case _, ok := <-pushPlansStream:
			if !ok {
				configStreamClosed = true
			}
		case _, ok := <-eventStream:
			if !ok {
				eventStreamClosed = true
			}
		case _, ok := <-warningsStream:
			if !ok {
				warningsStreamClosed = true
			}
		case _, ok := <-errorStream:
			if !ok {
				errorStreamClosed = true
			}
		}
		if configStreamClosed && eventStreamClosed && warningsStreamClosed && errorStreamClosed {
			break
		}
	}
	return true
}

func getPrepareNextEvent(c <-chan []PushPlan, e <-chan Event, w <-chan Warnings) func() Event {
	timeOut := time.Tick(500 * time.Millisecond)

	return func() Event {
		for {
			select {
			case <-c:
			case event, ok := <-e:
				if ok {
					log.WithField("event", event).Debug("getNextEvent")
					return event
				}
				return ""
			case <-w:
			case <-timeOut:
				return ""
			}
		}
	}
}

var _ = Describe("PrepareSpace", func() {
	var (
		actor       *Actor
		fakeV7Actor *v7pushactionfakes.FakeV7Actor

		pushPlans          []PushPlan
		fakeManifestParser *v7pushactionfakes.FakeManifestParser

		spaceGUID string

		pushPlansStream <-chan []PushPlan
		eventStream     <-chan Event
		warningsStream  <-chan Warnings
		errorStream     <-chan error
	)

	BeforeEach(func() {
		actor, _, fakeV7Actor, _ = getTestPushActor()

		spaceGUID = "space"

		fakeManifestParser = new(v7pushactionfakes.FakeManifestParser)
	})

	AfterEach(func() {
		Eventually(PrepareSpaceStreamsDrainedAndClosed(pushPlansStream, eventStream, warningsStream, errorStream)).Should(BeTrue())
	})

	JustBeforeEach(func() {
		pushPlansStream, eventStream, warningsStream, errorStream = actor.PrepareSpace(pushPlans, fakeManifestParser)
	})

	When("there is a single push state and no manifest", func() {
		var appName = "app-name"

		When("Creating the app succeeds", func() {
			BeforeEach(func() {
				pushPlans = []PushPlan{{SpaceGUID: spaceGUID, Application: v7action.Application{Name: appName}}}
				fakeV7Actor.CreateApplicationInSpaceReturns(
					v7action.Application{Name: appName},
					v7action.Warnings{"create-app-warning"},
					nil,
				)
			})

			It("creates the app using the API", func() {
				Consistently(fakeV7Actor.SetSpaceManifestCallCount).Should(Equal(0))
				Eventually(getPrepareNextEvent(pushPlansStream, eventStream, warningsStream)).Should(Equal(CreatingApplication))
				Eventually(fakeV7Actor.CreateApplicationInSpaceCallCount).Should(Equal(1))
				actualApp, actualSpaceGUID := fakeV7Actor.CreateApplicationInSpaceArgsForCall(0)
				Expect(actualApp).To(Equal(v7action.Application{Name: appName}))
				Expect(actualSpaceGUID).To(Equal(spaceGUID))
				Eventually(warningsStream).Should(Receive(Equal(Warnings{"create-app-warning"})))
				Eventually(pushPlansStream).Should(Receive(ConsistOf(PushPlan{
					SpaceGUID: spaceGUID, Application: v7action.Application{Name: appName},
				})))
				Eventually(getPrepareNextEvent(pushPlansStream, eventStream, warningsStream)).Should(Equal(CreatedApplication))
			})
		})

		When("the app already exists", func() {
			BeforeEach(func() {
				pushPlans = []PushPlan{{SpaceGUID: spaceGUID, Application: v7action.Application{Name: appName}}}
				fakeV7Actor.CreateApplicationInSpaceReturns(
					v7action.Application{},
					v7action.Warnings{"create-app-warning"},
					actionerror.ApplicationAlreadyExistsError{},
				)
			})
			It("Sends already exists events", func() {
				Consistently(fakeV7Actor.SetSpaceManifestCallCount).Should(Equal(0))
				Eventually(getPrepareNextEvent(pushPlansStream, eventStream, warningsStream)).Should(Equal(SkippingApplicationCreation))
				Eventually(fakeV7Actor.CreateApplicationInSpaceCallCount).Should(Equal(1))
				actualApp, actualSpaceGUID := fakeV7Actor.CreateApplicationInSpaceArgsForCall(0)
				Expect(actualApp).To(Equal(v7action.Application{Name: appName}))
				Expect(actualSpaceGUID).To(Equal(spaceGUID))
				Eventually(warningsStream).Should(Receive(Equal(Warnings{"create-app-warning"})))
				Eventually(pushPlansStream).Should(Receive(ConsistOf(PushPlan{
					SpaceGUID: spaceGUID, Application: v7action.Application{Name: appName},
				})))
				Eventually(getPrepareNextEvent(pushPlansStream, eventStream, warningsStream)).Should(Equal(ApplicationAlreadyExists))
			})
		})

		When("creating the app fails", func() {
			BeforeEach(func() {
				pushPlans = []PushPlan{{SpaceGUID: spaceGUID, Application: v7action.Application{Name: appName}}}
				fakeV7Actor.CreateApplicationInSpaceReturns(
					v7action.Application{},
					v7action.Warnings{"create-app-warning"},
					errors.New("some-create-error"),
				)
			})

			It("Returns the error", func() {
				Consistently(fakeV7Actor.SetSpaceManifestCallCount).Should(Equal(0))
				Eventually(getPrepareNextEvent(pushPlansStream, eventStream, warningsStream)).Should(Equal(CreatingApplication))
				Eventually(fakeV7Actor.CreateApplicationInSpaceCallCount).Should(Equal(1))
				actualApp, actualSpaceGuid := fakeV7Actor.CreateApplicationInSpaceArgsForCall(0)
				Expect(actualApp.Name).To(Equal(appName))
				Expect(actualSpaceGuid).To(Equal(spaceGUID))
				Eventually(warningsStream).Should(Receive(Equal(Warnings{"create-app-warning"})))
				Eventually(errorStream).Should(Receive(Equal(errors.New("some-create-error"))))
				Consistently(getPrepareNextEvent(pushPlansStream, eventStream, warningsStream)).ShouldNot(Equal(ApplicationAlreadyExists))
			})
		})
	})

	When("There is a a manifest", func() {
		var (
			manifest = []byte("app manifest")
			appName1 = "app-name1"
			appName2 = "app-name2"
		)
		When("applying the manifest fails", func() {
			BeforeEach(func() {
				pushPlans = []PushPlan{{SpaceGUID: spaceGUID, Application: v7action.Application{Name: appName1}}}
				fakeManifestParser.ContainsManifestReturns(true)
				fakeManifestParser.RawAppManifestReturns(manifest, nil)
				fakeV7Actor.SetSpaceManifestReturns(v7action.Warnings{"apply-manifest-warnings"}, errors.New("some-error"))
			})

			It("returns the error and exits", func() {
				Consistently(fakeV7Actor.CreateApplicationInSpaceCallCount).Should(Equal(0))
				Eventually(getPrepareNextEvent(pushPlansStream, eventStream, warningsStream)).Should(Equal(ApplyManifest))
				Eventually(fakeV7Actor.SetSpaceManifestCallCount).Should(Equal(1))
				actualSpaceGuid, actualManifest := fakeV7Actor.SetSpaceManifestArgsForCall(0)
				Expect(actualSpaceGuid).To(Equal(spaceGUID))
				Expect(actualManifest).To(Equal(manifest))
				Eventually(warningsStream).Should(Receive(Equal(Warnings{"apply-manifest-warnings"})))
				Eventually(errorStream).Should(Receive(Equal(errors.New("some-error"))))
				Consistently(pushPlansStream).ShouldNot(Receive(ConsistOf(PushPlan{
					SpaceGUID: spaceGUID, Application: v7action.Application{Name: appName1},
				})))
				Consistently(getPrepareNextEvent(pushPlansStream, eventStream, warningsStream)).ShouldNot(Equal(ApplyManifestComplete))
			})
		})

		When("There is a single pushPlan", func() {

			When("noRoute flag is provided with a manifest", func() {

				When("When the app name is in the manifest", func() {
					BeforeEach(func() {
						pushPlans = []PushPlan{{SpaceGUID: spaceGUID, Application: v7action.Application{Name: appName1}, SkipRouteCreation: true}}
						fakeManifestParser.ContainsManifestReturns(true)
						fakeManifestParser.ApplyNoRouteOverrideReturns(nil)
					})

					It("it overrides the manifest for the app", func() {
						Eventually(fakeManifestParser.ApplyNoRouteOverrideCallCount).Should(Equal(1))
						actualNoRouteAppName, actualNoRoute := fakeManifestParser.ApplyNoRouteOverrideArgsForCall(0)
						Expect(actualNoRouteAppName).To(Equal(appName1))
						Expect(actualNoRoute).To(BeTrue())
					})
				})

				When("When the app name is not in the manifest", func() {
					BeforeEach(func() {
						pushPlans = []PushPlan{{SpaceGUID: spaceGUID, Application: v7action.Application{Name: appName1}, SkipRouteCreation: true}}
						fakeManifestParser.ContainsManifestReturns(true)
						fakeManifestParser.ApplyNoRouteOverrideReturns(manifestparser.AppNotInManifestError{Name: appName1})
					})

					It("it errors", func() {
						Eventually(fakeManifestParser.ApplyNoRouteOverrideCallCount).Should(Equal(1))
						actualNoRouteAppName, actualNoRoute := fakeManifestParser.ApplyNoRouteOverrideArgsForCall(0)
						Expect(actualNoRouteAppName).To(Equal(appName1))
						Expect(actualNoRoute).To(BeTrue())
						Eventually(errorStream).Should(Receive(MatchError(manifestparser.AppNotInManifestError{Name: appName1})))
					})
				})
			})

			BeforeEach(func() {
				pushPlans = []PushPlan{{SpaceGUID: spaceGUID, Application: v7action.Application{Name: appName1}}}
				fakeManifestParser.ContainsManifestReturns(true)
				fakeManifestParser.RawAppManifestReturns(manifest, nil)
				fakeV7Actor.SetSpaceManifestReturns(v7action.Warnings{"apply-manifest-warnings"}, nil)
			})

			It("applies the app specific manifest", func() {
				Consistently(fakeV7Actor.CreateApplicationInSpaceCallCount).Should(Equal(0))
				Eventually(getPrepareNextEvent(pushPlansStream, eventStream, warningsStream)).Should(Equal(ApplyManifest))
				Eventually(fakeManifestParser.RawAppManifestCallCount).Should(Equal(1))
				actualAppName := fakeManifestParser.RawAppManifestArgsForCall(0)
				Expect(actualAppName).To(Equal(appName1))
				Eventually(fakeV7Actor.SetSpaceManifestCallCount).Should(Equal(1))
				actualSpaceGUID, actualManifest := fakeV7Actor.SetSpaceManifestArgsForCall(0)
				Expect(actualManifest).To(Equal(manifest))
				Expect(actualSpaceGUID).To(Equal(spaceGUID))
				Eventually(warningsStream).Should(Receive(Equal(Warnings{"apply-manifest-warnings"})))
				Eventually(pushPlansStream).Should(Receive(ConsistOf(PushPlan{
					SpaceGUID: spaceGUID, Application: v7action.Application{Name: appName1},
				})))
				Eventually(getPrepareNextEvent(pushPlansStream, eventStream, warningsStream)).Should(Equal(ApplyManifestComplete))
			})
		})

		When("There are multiple push states", func() {
			BeforeEach(func() {
				pushPlans = []PushPlan{
					{SpaceGUID: spaceGUID, Application: v7action.Application{Name: appName1}},
					{SpaceGUID: spaceGUID, Application: v7action.Application{Name: appName2}},
				}
				fakeManifestParser.ContainsManifestReturns(true)
				fakeManifestParser.FullRawManifestReturns(manifest)
				fakeV7Actor.SetSpaceManifestReturns(v7action.Warnings{"apply-manifest-warnings"}, nil)
			})

			It("Applies the entire manifest", func() {
				Consistently(fakeV7Actor.CreateApplicationInSpaceCallCount).Should(Equal(0))
				Consistently(fakeManifestParser.RawAppManifestCallCount).Should(Equal(0))
				Eventually(getPrepareNextEvent(pushPlansStream, eventStream, warningsStream)).Should(Equal(ApplyManifest))
				Eventually(fakeManifestParser.FullRawManifestCallCount).Should(Equal(1))
				Eventually(fakeV7Actor.SetSpaceManifestCallCount).Should(Equal(1))
				actualSpaceGUID, actualManifest := fakeV7Actor.SetSpaceManifestArgsForCall(0)
				Expect(actualManifest).To(Equal(manifest))
				Expect(actualSpaceGUID).To(Equal(spaceGUID))
				Eventually(warningsStream).Should(Receive(Equal(Warnings{"apply-manifest-warnings"})))
				Eventually(pushPlansStream).Should(Receive(ConsistOf(
					PushPlan{SpaceGUID: spaceGUID, Application: v7action.Application{Name: appName1}},
					PushPlan{SpaceGUID: spaceGUID, Application: v7action.Application{Name: appName2}},
				)))
				Eventually(getPrepareNextEvent(pushPlansStream, eventStream, warningsStream)).Should(Equal(ApplyManifestComplete))
			})
		})
	})
})
