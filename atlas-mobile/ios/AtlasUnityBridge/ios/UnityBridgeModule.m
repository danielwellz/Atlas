#import <Foundation/Foundation.h>
#import <React/RCTBridgeModule.h>
#import <React/RCTEventEmitter.h>
#import <React/RCTUtils.h>
#import <UIKit/UIKit.h>
#import <objc/message.h>

static NSString *const kUnityBridgeEventName = @"UnityMessage";
static NSString *const kUnityBridgeGameObject = @"AtlasBridge";
static NSString *const kUnityBridgeReceiverMethod = @"OnReactNativeMessage";
static NSString *const kUnityLifecycleTopic = @"unity.lifecycle";
static NSString *const kUnityStateTopic = @"unity.state";
static NSString *const kUnityReadyTopic = @"unity.ready";
static NSUInteger const kMaxPendingUnityMessages = 64;

@class UnityBridgeModule;

static __weak UnityBridgeModule *sUnityBridgeModule = nil;

static NSString *AtlasUnityJSONString(NSDictionary<NSString *, id> *payload)
{
  NSError *serializationError = nil;
  NSData *data = [NSJSONSerialization dataWithJSONObject:payload options:0 error:&serializationError];
  if (serializationError != nil || data == nil) {
    return @"{}";
  }

  NSString *serialized = [[NSString alloc] initWithData:data encoding:NSUTF8StringEncoding];
  return serialized ?: @"{}";
}

@interface AtlasUnityFallbackViewController : UIViewController
@property (nonatomic, copy, nullable) dispatch_block_t onClose;
@end

@implementation AtlasUnityFallbackViewController

- (void)viewDidLoad
{
  [super viewDidLoad];

  self.view.backgroundColor = [UIColor colorWithRed:2.0 / 255.0 green:6.0 / 255.0 blue:23.0 / 255.0 alpha:1.0];

  UILabel *titleLabel = [[UILabel alloc] initWithFrame:CGRectZero];
  titleLabel.translatesAutoresizingMaskIntoConstraints = NO;
  titleLabel.text = @"Unity framework is not packaged yet.";
  titleLabel.textColor = [UIColor whiteColor];
  titleLabel.font = [UIFont boldSystemFontOfSize:21.0];
  titleLabel.textAlignment = NSTextAlignmentCenter;

  UILabel *subtitleLabel = [[UILabel alloc] initWithFrame:CGRectZero];
  subtitleLabel.translatesAutoresizingMaskIntoConstraints = NO;
  subtitleLabel.text = @"Export atlas-unity as Unity Library to enable runtime scenes.";
  subtitleLabel.textColor = [UIColor colorWithRed:148.0 / 255.0 green:163.0 / 255.0 blue:184.0 / 255.0 alpha:1.0];
  subtitleLabel.font = [UIFont systemFontOfSize:15.0 weight:UIFontWeightRegular];
  subtitleLabel.textAlignment = NSTextAlignmentCenter;
  subtitleLabel.numberOfLines = 0;

  UIButton *closeButton = [UIButton buttonWithType:UIButtonTypeSystem];
  closeButton.translatesAutoresizingMaskIntoConstraints = NO;
  [closeButton setTitle:@"Close" forState:UIControlStateNormal];
  closeButton.titleLabel.font = [UIFont boldSystemFontOfSize:17.0];
  [closeButton setTitleColor:[UIColor whiteColor] forState:UIControlStateNormal];
  closeButton.backgroundColor = [UIColor colorWithRed:15.0 / 255.0 green:118.0 / 255.0 blue:110.0 / 255.0 alpha:1.0];
  closeButton.layer.cornerRadius = 12.0;
  [closeButton addTarget:self action:@selector(handleCloseTap) forControlEvents:UIControlEventTouchUpInside];

  [self.view addSubview:titleLabel];
  [self.view addSubview:subtitleLabel];
  [self.view addSubview:closeButton];

  [NSLayoutConstraint activateConstraints:@[
    [titleLabel.leadingAnchor constraintEqualToAnchor:self.view.leadingAnchor constant:24.0],
    [titleLabel.trailingAnchor constraintEqualToAnchor:self.view.trailingAnchor constant:-24.0],
    [titleLabel.centerYAnchor constraintEqualToAnchor:self.view.centerYAnchor constant:-34.0],

    [subtitleLabel.leadingAnchor constraintEqualToAnchor:self.view.leadingAnchor constant:24.0],
    [subtitleLabel.trailingAnchor constraintEqualToAnchor:self.view.trailingAnchor constant:-24.0],
    [subtitleLabel.topAnchor constraintEqualToAnchor:titleLabel.bottomAnchor constant:10.0],

    [closeButton.centerXAnchor constraintEqualToAnchor:self.view.centerXAnchor],
    [closeButton.bottomAnchor constraintEqualToAnchor:self.view.safeAreaLayoutGuide.bottomAnchor constant:-28.0],
    [closeButton.widthAnchor constraintEqualToConstant:160.0],
    [closeButton.heightAnchor constraintEqualToConstant:50.0],
  ]];
}

- (void)handleCloseTap
{
  if (self.onClose != nil) {
    self.onClose();
  }
}

@end

@interface UnityBridgeModule : RCTEventEmitter <RCTBridgeModule>
@property (nonatomic, assign) BOOL hasListeners;
@property (nonatomic, strong, nullable) AtlasUnityFallbackViewController *fallbackController;
@property (nonatomic, strong, nullable) id unityFrameworkInstance;
@property (nonatomic, assign) BOOL unityRuntimeStarted;
@property (nonatomic, assign) BOOL unityOpenRequested;
@property (nonatomic, strong) NSMutableArray<NSString *> *pendingUnityMessages;
@property (nonatomic, assign) NSInteger openCount;
@property (nonatomic, assign) NSInteger closeCount;
@end

@implementation UnityBridgeModule

RCT_EXPORT_MODULE(UnityBridgeModule)

+ (BOOL)requiresMainQueueSetup
{
  return YES;
}

- (instancetype)init
{
  self = [super init];
  if (self != nil) {
    sUnityBridgeModule = self;
    _pendingUnityMessages = [NSMutableArray array];
    [[NSNotificationCenter defaultCenter] addObserver:self
                                             selector:@selector(handleApplicationDidEnterBackground)
                                                 name:UIApplicationDidEnterBackgroundNotification
                                               object:nil];
  }
  return self;
}

- (void)dealloc
{
  [[NSNotificationCenter defaultCenter] removeObserver:self];
}

- (NSArray<NSString *> *)supportedEvents
{
  return @[ kUnityBridgeEventName ];
}

- (void)startObserving
{
  self.hasListeners = YES;
}

- (void)stopObserving
{
  self.hasListeners = NO;
}

RCT_REMAP_METHOD(openUnity,
                 openUnityWithResolver:(RCTPromiseResolveBlock)resolve
                 rejecter:(RCTPromiseRejectBlock)reject)
{
  dispatch_async(dispatch_get_main_queue(), ^{
    @try {
      self.unityOpenRequested = YES;
      [self emitUnityState:@"loading" mode:@"native" reason:@""];
      BOOL launchedUnityRuntime = [self launchUnityRuntimeIfAvailable];
      if (!launchedUnityRuntime) {
        [self presentFallbackController];
      }

      self.unityOpenRequested = NO;
      self.openCount += 1;
      [self emitLifecycleState:@"opened" mode:(launchedUnityRuntime ? @"unity" : @"fallback")];
      if (launchedUnityRuntime) {
        [self emitUnityState:@"loaded" mode:@"unity" reason:@""];
        [self drainPendingUnityMessages];
      } else {
        [self emitUnityState:@"failed" mode:@"fallback" reason:@"unity_framework_unavailable"];
        [self clearPendingUnityMessages];
      }
      resolve(nil);
    } @catch (NSException *exception) {
      self.unityOpenRequested = NO;
      [self emitUnityState:@"failed" mode:@"native" reason:@"open_exception"];
      reject(@"E_OPEN_UNITY", exception.reason, nil);
    }
  });
}

RCT_REMAP_METHOD(closeUnity,
                 closeUnityWithResolver:(RCTPromiseResolveBlock)resolve
                 rejecter:(RCTPromiseRejectBlock)reject)
{
  dispatch_async(dispatch_get_main_queue(), ^{
    @try {
      [self closeUnityRuntimeAndFallback];
      self.closeCount += 1;
      [self emitLifecycleState:@"closed" mode:@"native"];
      [self emitUnityState:@"closed" mode:@"native" reason:@""];
      resolve(nil);
    } @catch (NSException *exception) {
      reject(@"E_CLOSE_UNITY", exception.reason, nil);
    }
  });
}

RCT_REMAP_METHOD(sendMessageToUnity,
                 sendMessageToUnityWithTopic:(NSString *)topic
                 payload:(NSString *)payload
                 resolver:(RCTPromiseResolveBlock)resolve
                 rejecter:(RCTPromiseRejectBlock)reject)
{
  dispatch_async(dispatch_get_main_queue(), ^{
    @try {
      NSString *encodedPayload = AtlasUnityJSONString(@{
        @"topic" : topic ?: @"",
        @"payload" : payload ?: @"",
      });

      BOOL deliveredToUnity = [self sendMessageToUnityRuntime:encodedPayload];
      if (!deliveredToUnity) {
        if (self.unityOpenRequested || self.unityRuntimeStarted) {
          [self enqueuePendingUnityMessage:encodedPayload];
        } else {
          [self emitUnityEventTopic:@"unity.echo" payload:encodedPayload];
        }
      }

      resolve(nil);
    } @catch (NSException *exception) {
      reject(@"E_SEND_UNITY_MESSAGE", exception.reason, nil);
    }
  });
}

RCT_REMAP_METHOD(receiveMessageFromUnity,
                 receiveMessageFromUnityWithResolver:(RCTPromiseResolveBlock)resolve
                 rejecter:(RCTPromiseRejectBlock)reject)
{
  resolve(@YES);
}

- (void)handleMessageFromUnityTopic:(NSString *)topic payload:(NSString *)payload
{
  [self emitUnityEventTopic:topic payload:payload];
  if ([topic isEqualToString:kUnityReadyTopic]) {
    [self emitUnityState:@"loaded" mode:@"unity" reason:@""];
    [self drainPendingUnityMessages];
  }
}

- (void)emitUnityEventTopic:(NSString *)topic payload:(NSString *)payload
{
  if (!self.hasListeners) {
    return;
  }

  [self sendEventWithName:kUnityBridgeEventName
                     body:@{
                       @"topic" : topic ?: @"",
                       @"payload" : payload ?: @"",
                     }];
}

- (void)emitLifecycleState:(NSString *)state mode:(NSString *)mode
{
  NSString *payload = AtlasUnityJSONString(@{
    @"state" : state,
    @"mode" : mode,
    @"openCount" : @(self.openCount),
    @"closeCount" : @(self.closeCount),
  });

  [self emitUnityEventTopic:kUnityLifecycleTopic payload:payload];
}

- (void)emitUnityState:(NSString *)state mode:(NSString *)mode reason:(NSString *)reason
{
  NSString *payload = AtlasUnityJSONString(@{
    @"state" : state ?: @"",
    @"mode" : mode ?: @"",
    @"reason" : reason ?: @"",
    @"openCount" : @(self.openCount),
    @"closeCount" : @(self.closeCount),
  });

  [self emitUnityEventTopic:kUnityStateTopic payload:payload];
}

- (nullable id)loadUnityFrameworkIfNeeded
{
  if (self.unityFrameworkInstance != nil) {
    return self.unityFrameworkInstance;
  }

  NSString *frameworkPath = [NSBundle.mainBundle.bundlePath stringByAppendingPathComponent:@"Frameworks/UnityFramework.framework"];
  NSBundle *frameworkBundle = [NSBundle bundleWithPath:frameworkPath];
  if (frameworkBundle == nil) {
    return nil;
  }

  NSError *bundleLoadError = nil;
  if (!frameworkBundle.loaded && ![frameworkBundle loadAndReturnError:&bundleLoadError]) {
    NSLog(@"[UnityBridgeModule] Failed to load UnityFramework.bundle: %@", bundleLoadError.localizedDescription);
    return nil;
  }

  Class unityFrameworkClass = NSClassFromString(@"UnityFramework");
  SEL getInstanceSelector = NSSelectorFromString(@"getInstance");
  if (unityFrameworkClass == Nil || ![unityFrameworkClass respondsToSelector:getInstanceSelector]) {
    return nil;
  }

  id unityFramework = ((id (*)(id, SEL))objc_msgSend)(unityFrameworkClass, getInstanceSelector);
  self.unityFrameworkInstance = unityFramework;
  return unityFramework;
}

- (BOOL)launchUnityRuntimeIfAvailable
{
  id unityFramework = [self loadUnityFrameworkIfNeeded];
  if (unityFramework == nil) {
    return NO;
  }

  SEL setDataBundleIdSelector = NSSelectorFromString(@"setDataBundleId:");
  if ([unityFramework respondsToSelector:setDataBundleIdSelector]) {
    ((void (*)(id, SEL, const char *))objc_msgSend)(unityFramework, setDataBundleIdSelector, "com.unity3d.framework");
  }

  if (!self.unityRuntimeStarted) {
    SEL runEmbeddedSelector = NSSelectorFromString(@"runEmbeddedWithArgc:argv:appLaunchOpts:");
    if (![unityFramework respondsToSelector:runEmbeddedSelector]) {
      return NO;
    }

    ((void (*)(id, SEL, int, char **, NSDictionary *))objc_msgSend)(unityFramework, runEmbeddedSelector, 0, NULL, nil);
    self.unityRuntimeStarted = YES;
  }

  SEL showUnityWindowSelector = NSSelectorFromString(@"showUnityWindow");
  if (![unityFramework respondsToSelector:showUnityWindowSelector]) {
    return NO;
  }

  ((void (*)(id, SEL))objc_msgSend)(unityFramework, showUnityWindowSelector);
  return YES;
}

- (BOOL)sendMessageToUnityRuntime:(NSString *)encodedPayload
{
  id unityFramework = [self loadUnityFrameworkIfNeeded];
  if (unityFramework == nil || !self.unityRuntimeStarted) {
    return NO;
  }

  SEL sendMessageSelector = NSSelectorFromString(@"sendMessageToGOWithName:functionName:message:");
  if (![unityFramework respondsToSelector:sendMessageSelector]) {
    return NO;
  }

  ((void (*)(id, SEL, const char *, const char *, const char *))objc_msgSend)(
      unityFramework,
      sendMessageSelector,
      [kUnityBridgeGameObject UTF8String],
      [kUnityBridgeReceiverMethod UTF8String],
      [encodedPayload UTF8String]);
  return YES;
}

- (void)presentFallbackController
{
  UIViewController *presentingViewController = RCTPresentedViewController();
  if (presentingViewController == nil) {
    return;
  }

  if (self.fallbackController == nil) {
    __weak __typeof(self) weakSelf = self;
    AtlasUnityFallbackViewController *fallbackController = [[AtlasUnityFallbackViewController alloc] init];
    fallbackController.modalPresentationStyle = UIModalPresentationFullScreen;
    fallbackController.onClose = ^{
      [weakSelf closeUnityRuntimeAndFallback];
      weakSelf.closeCount += 1;
      [weakSelf emitLifecycleState:@"closed" mode:@"fallback"];
      [weakSelf emitUnityState:@"closed" mode:@"fallback" reason:@""];
    };

    self.fallbackController = fallbackController;
  }

  if (presentingViewController == self.fallbackController ||
      presentingViewController.presentedViewController == self.fallbackController) {
    return;
  }

  [presentingViewController presentViewController:self.fallbackController animated:YES completion:nil];
}

- (void)dismissFallbackController
{
  if (self.fallbackController == nil) {
    return;
  }

  if (self.fallbackController.presentingViewController != nil) {
    [self.fallbackController dismissViewControllerAnimated:YES completion:nil];
  }

  self.fallbackController = nil;
}

- (void)closeUnityRuntimeAndFallback
{
  [self dismissFallbackController];

  if (self.unityFrameworkInstance != nil) {
    SEL unloadSelector = NSSelectorFromString(@"unloadApplication");
    if ([self.unityFrameworkInstance respondsToSelector:unloadSelector]) {
      ((void (*)(id, SEL))objc_msgSend)(self.unityFrameworkInstance, unloadSelector);
    }
  }

  self.unityFrameworkInstance = nil;
  self.unityRuntimeStarted = NO;
  self.unityOpenRequested = NO;
  [self clearPendingUnityMessages];
}

- (void)enqueuePendingUnityMessage:(NSString *)encodedPayload
{
  if (encodedPayload.length == 0) {
    return;
  }

  if (self.pendingUnityMessages.count >= kMaxPendingUnityMessages) {
    [self.pendingUnityMessages removeObjectAtIndex:0];
  }

  [self.pendingUnityMessages addObject:encodedPayload];
}

- (void)drainPendingUnityMessages
{
  while (self.pendingUnityMessages.count > 0) {
    NSString *encodedPayload = self.pendingUnityMessages.firstObject;
    [self.pendingUnityMessages removeObjectAtIndex:0];
    if (![self sendMessageToUnityRuntime:encodedPayload]) {
      [self enqueuePendingUnityMessage:encodedPayload];
      return;
    }
  }
}

- (void)clearPendingUnityMessages
{
  [self.pendingUnityMessages removeAllObjects];
}

- (void)handleApplicationDidEnterBackground
{
  if (!self.unityRuntimeStarted && self.fallbackController == nil) {
    return;
  }

  [self closeUnityRuntimeAndFallback];
  self.closeCount += 1;
  [self emitLifecycleState:@"closed" mode:@"background"];
  [self emitUnityState:@"closed" mode:@"background" reason:@"app_backgrounded"];
}

@end

void AtlasUnitySendMessageToReact(const char *topic, const char *payload)
{
  NSString *topicValue = topic != NULL ? [NSString stringWithUTF8String:topic] : @"unity.message";
  NSString *payloadValue = payload != NULL ? [NSString stringWithUTF8String:payload] : @"";

  if (topicValue == nil) {
    topicValue = @"unity.message";
  }

  if (payloadValue == nil) {
    payloadValue = @"";
  }

  dispatch_async(dispatch_get_main_queue(), ^{
    [sUnityBridgeModule handleMessageFromUnityTopic:topicValue payload:payloadValue];
  });
}
