#import <Foundation/Foundation.h>
#import <float.h>
#import <React/RCTBridgeModule.h>
#import <React/RCTEventEmitter.h>

static NSString *const kFormCheckPoseFrameEvent = @"FormCheckPoseFrame";

static double AtlasClampScore(double value)
{
  if (value < 0.0) {
    return 0.0;
  }
  if (value > 100.0) {
    return 100.0;
  }
  return value;
}

static double AtlasRound1(double value)
{
  return round(value * 10.0) / 10.0;
}

@interface FormCheckPoseModule : RCTEventEmitter <RCTBridgeModule>
@property (nonatomic, assign) BOOL hasListeners;
@property (nonatomic, assign) BOOL running;
@property (nonatomic, assign) double phase;
@property (nonatomic, copy) NSString *movementType;
@property (nonatomic, strong) NSTimer *frameTimer;
@property (nonatomic, strong) NSMutableArray<NSDictionary<NSString *, NSNumber *> *> *frames;
@end

@implementation FormCheckPoseModule

RCT_EXPORT_MODULE(FormCheckPoseModule)

+ (BOOL)requiresMainQueueSetup
{
  return YES;
}

- (instancetype)init
{
  self = [super init];
  if (self != nil) {
    _movementType = @"squat";
    _frames = [NSMutableArray array];
  }
  return self;
}

- (NSArray<NSString *> *)supportedEvents
{
  return @[ kFormCheckPoseFrameEvent ];
}

- (void)startObserving
{
  self.hasListeners = YES;
}

- (void)stopObserving
{
  self.hasListeners = NO;
}

RCT_REMAP_METHOD(startDetection,
                 startDetectionWithMovementType:(NSString *)movementType
                 resolver:(RCTPromiseResolveBlock)resolve
                 rejecter:(RCTPromiseRejectBlock)reject)
{
  dispatch_async(dispatch_get_main_queue(), ^{
    self.movementType = movementType.length > 0 ? movementType : @"squat";
    self.phase = 0.0;
    self.running = YES;
    [self.frames removeAllObjects];

    if (self.frameTimer != nil) {
      [self.frameTimer invalidate];
      self.frameTimer = nil;
    }

    self.frameTimer =
        [NSTimer scheduledTimerWithTimeInterval:0.12
                                         target:self
                                       selector:@selector(emitSyntheticFrame)
                                       userInfo:nil
                                        repeats:YES];
    resolve(nil);
  });
}

RCT_REMAP_METHOD(stopDetection,
                 stopDetectionWithResolver:(RCTPromiseResolveBlock)resolve
                 rejecter:(RCTPromiseRejectBlock)reject)
{
  dispatch_async(dispatch_get_main_queue(), ^{
    self.running = NO;
    if (self.frameTimer != nil) {
      [self.frameTimer invalidate];
      self.frameTimer = nil;
    }

    NSDictionary<NSString *, id> *summary = [self buildSummary];
    [self.frames removeAllObjects];
    resolve(summary);
  });
}

- (void)invalidate
{
  self.running = NO;
  if (self.frameTimer != nil) {
    [self.frameTimer invalidate];
    self.frameTimer = nil;
  }

  [super invalidate];
}

- (void)emitSyntheticFrame
{
  if (!self.running) {
    return;
  }

  double depth = (sin(self.phase) + 1.0) / 2.0;
  double sway = sin(self.phase * 0.5);

  double leftKnee = 173.0 - depth * 92.0 + sway * 3.0;
  double rightKnee = 171.0 - depth * 90.0 - sway * 3.0;
  double leftHip = 169.0 - depth * 78.0 + sway * 2.0;
  double rightHip = 167.0 - depth * 80.0 - sway * 2.0;

  self.phase += 0.22;

  NSDictionary<NSString *, NSNumber *> *frame = @{
    @"timestampMs" : @([[NSDate date] timeIntervalSince1970] * 1000.0),
    @"leftKneeDeg" : @(leftKnee),
    @"rightKneeDeg" : @(rightKnee),
    @"leftHipDeg" : @(leftHip),
    @"rightHipDeg" : @(rightHip),
  };

  [self.frames addObject:frame];
  if (self.frames.count > 900) {
    [self.frames removeObjectAtIndex:0];
  }

  if (self.hasListeners) {
    [self sendEventWithName:kFormCheckPoseFrameEvent body:frame];
  }
}

- (NSDictionary<NSString *, id> *)buildSummary
{
  if (self.frames.count < 5) {
    return @{
      @"movementType" : self.movementType ?: @"squat",
      @"sampleCount" : @(self.frames.count),
      @"repetitionCount" : @0,
      @"rangeOfMotionDegrees" : @0,
      @"rangeOfMotionScore" : @0,
      @"kneeTrackingScore" : @0,
      @"symmetryScore" : @0,
      @"overallScore" : @0,
      @"feedback" : @[ @"Record a longer set for a reliable form check." ],
      @"minLeftKneeDeg" : @0,
      @"minRightKneeDeg" : @0,
      @"maxLeftKneeDeg" : @0,
      @"maxRightKneeDeg" : @0,
    };
  }

  double minLeft = DBL_MAX;
  double maxLeft = -DBL_MAX;
  double minRight = DBL_MAX;
  double maxRight = -DBL_MAX;
  double kneeGapTotal = 0.0;
  NSInteger reps = 0;
  BOOL inBottom = NO;

  for (NSDictionary<NSString *, NSNumber *> *frame in self.frames) {
    double leftKnee = frame[@"leftKneeDeg"].doubleValue;
    double rightKnee = frame[@"rightKneeDeg"].doubleValue;

    minLeft = MIN(minLeft, leftKnee);
    maxLeft = MAX(maxLeft, leftKnee);
    minRight = MIN(minRight, rightKnee);
    maxRight = MAX(maxRight, rightKnee);

    kneeGapTotal += fabs(leftKnee - rightKnee);

    double averageKnee = (leftKnee + rightKnee) / 2.0;
    double kneeDepth = 180.0 - averageKnee;
    if (!inBottom && kneeDepth >= 55.0) {
      inBottom = YES;
      reps += 1;
    } else if (inBottom && kneeDepth <= 24.0) {
      inBottom = NO;
    }
  }

  double leftROM = maxLeft - minLeft;
  double rightROM = maxRight - minRight;
  double averageROM = (leftROM + rightROM) / 2.0;
  double averageKneeGap = kneeGapTotal / (double)self.frames.count;

  double romDelta = fabs(leftROM - rightROM);
  double depthDelta = fabs(minLeft - minRight);

  NSInteger rangeOfMotionScore = (NSInteger)AtlasClampScore(round((averageROM / 95.0) * 100.0));
  NSInteger kneeTrackingScore = (NSInteger)AtlasClampScore(round(100.0 - averageKneeGap * 2.2));
  NSInteger symmetryScore = (NSInteger)AtlasClampScore(round(100.0 - romDelta * 2.4 - depthDelta * 1.2));
  NSInteger overallScore =
      (NSInteger)AtlasClampScore(round(rangeOfMotionScore * 0.45 + kneeTrackingScore * 0.30 + symmetryScore * 0.25));

  NSMutableArray<NSString *> *feedback = [NSMutableArray array];
  if (averageROM < 50.0) {
    [feedback addObject:@"Increase depth to improve squat range of motion."];
  }
  if (averageKneeGap > 15.0) {
    [feedback addObject:@"Keep knees tracking evenly over the mid-foot."];
  }
  if (symmetryScore < 70) {
    [feedback addObject:@"Work on left/right symmetry and controlled descent."];
  }
  if (feedback.count == 0) {
    [feedback addObject:@"Solid rep quality across depth, tracking, and symmetry."];
  }

  return @{
    @"movementType" : self.movementType ?: @"squat",
    @"sampleCount" : @(self.frames.count),
    @"repetitionCount" : @(reps),
    @"rangeOfMotionDegrees" : @(AtlasRound1(averageROM)),
    @"rangeOfMotionScore" : @(rangeOfMotionScore),
    @"kneeTrackingScore" : @(kneeTrackingScore),
    @"symmetryScore" : @(symmetryScore),
    @"overallScore" : @(overallScore),
    @"feedback" : feedback,
    @"minLeftKneeDeg" : @(AtlasRound1(minLeft)),
    @"minRightKneeDeg" : @(AtlasRound1(minRight)),
    @"maxLeftKneeDeg" : @(AtlasRound1(maxLeft)),
    @"maxRightKneeDeg" : @(AtlasRound1(maxRight)),
  };
}

@end
