import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { ActivityIndicator, Platform, ScrollView, StyleSheet, Text, View } from 'react-native';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import type { BottomTabNavigationProp } from '@react-navigation/bottom-tabs';
import { useNavigation } from '@react-navigation/native';
import { Camera } from 'react-native-camera-kit';
import {
  PERMISSIONS,
  RESULTS,
  check,
  openSettings,
  request,
  type Permission,
} from 'react-native-permissions';
import {
  fetchConsents,
  grantConsent,
  type PrivacyConsentType,
} from '../../api/services/consentService';
import {
  uploadFormCheckResult,
  type FormCheckMovementType,
} from '../../api/services/formCheckService';
import {
  summarizePoseFrames,
  type FormCheckPoseSummary,
  type PoseAngleFrame,
} from '../../features/formCheck/scoring';
import { hasFormCheckUploadEntitlement } from '../../features/entitlements';
import {
  resetFormCheckPoseRuntime,
  startFormCheckDetection,
  stopFormCheckDetection,
  subscribeToPoseFrames,
} from '../../native/formCheckPose';
import { isNetworkOnline } from '../../network/onlineManager';
import type { MainTabParamList } from '../../navigation/types';
import { useAuth } from '../../state/AuthContext';
import { useMockMode } from '../../state/MockModeContext';
import { Button, Card } from '../../ui';

const CAMERA_PERMISSION: Permission =
  Platform.OS === 'ios' ? PERMISSIONS.IOS.CAMERA : PERMISSIONS.ANDROID.CAMERA;

const MOVEMENT_TYPE: FormCheckMovementType = 'squat';

function hasActiveConsent(
  consents: Awaited<ReturnType<typeof fetchConsents>> | undefined,
  consentType: PrivacyConsentType,
): boolean {
  return Boolean(consents?.some(consent => consent.consentType === consentType && !consent.revokedAt));
}

function summarizeLiveFrames(frames: PoseAngleFrame[]): FormCheckPoseSummary {
  return summarizePoseFrames({
    movementType: MOVEMENT_TYPE,
    frames,
  });
}

export function FormCheckScreen(): React.JSX.Element {
  const navigation = useNavigation<BottomTabNavigationProp<MainTabParamList>>();
  const { session } = useAuth();
  const { isMockMode } = useMockMode();
  const queryClient = useQueryClient();

  const [cameraPermission, setCameraPermission] = useState<string>('checking');
  const [isDetecting, setIsDetecting] = useState(false);
  const [latestFrame, setLatestFrame] = useState<PoseAngleFrame | null>(null);
  const [capturedFrames, setCapturedFrames] = useState<PoseAngleFrame[]>([]);
  const [resultSummary, setResultSummary] = useState<FormCheckPoseSummary | null>(null);
  const [recordingStartedAt, setRecordingStartedAt] = useState<string | null>(null);
  const [recordingEndedAt, setRecordingEndedAt] = useState<string | null>(null);
  const [statusMessage, setStatusMessage] = useState<string | undefined>();
  const [errorMessage, setErrorMessage] = useState<string | undefined>();

  const detectionActiveRef = useRef(false);
  useEffect(() => {
    detectionActiveRef.current = isDetecting;
  }, [isDetecting]);

  const consentsQuery = useQuery({
    queryKey: ['privacy-consents', session?.user.id, isMockMode],
    queryFn: () => fetchConsents(session!.tokens.accessToken, isMockMode),
    enabled: Boolean(session?.tokens.accessToken),
  });

  const grantLocalConsentMutation = useMutation({
    mutationFn: async () => {
      if (!session?.tokens.accessToken) {
        throw new Error('Missing authentication token.');
      }

      return grantConsent(
        {
          accessToken: session.tokens.accessToken,
          consentType: 'form_check_local',
          metadataJson: {
            source: 'form_check_screen',
          },
        },
        isMockMode,
      );
    },
    onSuccess: async () => {
      setErrorMessage(undefined);
      setStatusMessage('Local form-check consent enabled.');
      await queryClient.invalidateQueries({
        queryKey: ['privacy-consents', session?.user.id, isMockMode],
      });
    },
    onError: error => {
      setStatusMessage(undefined);
      setErrorMessage(error instanceof Error ? error.message : 'Unable to enable local form check.');
    },
  });

  const grantUploadConsentMutation = useMutation({
    mutationFn: async () => {
      if (!session?.tokens.accessToken) {
        throw new Error('Missing authentication token.');
      }

      return grantConsent(
        {
          accessToken: session.tokens.accessToken,
          consentType: 'form_check_upload',
          metadataJson: {
            source: 'form_check_screen_upload',
          },
        },
        isMockMode,
      );
    },
    onSuccess: async () => {
      setErrorMessage(undefined);
      setStatusMessage('Upload consent enabled. Upload only happens when you tap Upload to Coach.');
      await queryClient.invalidateQueries({
        queryKey: ['privacy-consents', session?.user.id, isMockMode],
      });
    },
    onError: error => {
      setStatusMessage(undefined);
      setErrorMessage(error instanceof Error ? error.message : 'Unable to enable upload consent.');
    },
  });

  const uploadMutation = useMutation({
    mutationFn: async () => {
      if (!session?.tokens.accessToken) {
        throw new Error('Missing authentication token.');
      }

      if (!recordingStartedAt || !recordingEndedAt || !resultSummary) {
        throw new Error('No completed form-check result available for upload.');
      }

      return uploadFormCheckResult({
        accessToken: session.tokens.accessToken,
        movementType: MOVEMENT_TYPE,
        recordingStartedAt,
        recordingEndedAt,
        summary: {
          overallScore: resultSummary.overallScore,
          rangeOfMotionScore: resultSummary.rangeOfMotionScore,
          kneeTrackingScore: resultSummary.kneeTrackingScore,
          symmetryScore: resultSummary.symmetryScore,
          rangeOfMotionDegrees: resultSummary.rangeOfMotionDegrees,
          repetitionCount: resultSummary.repetitionCount,
          feedback: resultSummary.feedback,
        },
        metadataJson: {
          sampleCount: resultSummary.sampleCount,
          minLeftKneeDeg: resultSummary.minLeftKneeDeg,
          minRightKneeDeg: resultSummary.minRightKneeDeg,
          maxLeftKneeDeg: resultSummary.maxLeftKneeDeg,
          maxRightKneeDeg: resultSummary.maxRightKneeDeg,
        },
      });
    },
    onSuccess: upload => {
      setErrorMessage(undefined);
      setStatusMessage(`Uploaded to coach review queue: ${upload.id}`);
    },
    onError: error => {
      setStatusMessage(undefined);
      setErrorMessage(error instanceof Error ? error.message : 'Unable to upload form-check result.');
    },
  });

  const uploadEntitled = hasFormCheckUploadEntitlement(session?.user);
  const localConsentGranted = hasActiveConsent(consentsQuery.data, 'form_check_local');
  const uploadConsentGranted = hasActiveConsent(consentsQuery.data, 'form_check_upload');

  const liveSummary = useMemo(() => summarizeLiveFrames(capturedFrames), [capturedFrames]);

  const checkPermission = useCallback(async () => {
    const status = await check(CAMERA_PERMISSION);
    setCameraPermission(status);
  }, []);

  useEffect(() => {
    checkPermission().catch(() => {
      setCameraPermission(RESULTS.UNAVAILABLE);
    });
  }, [checkPermission]);

  useEffect(() => {
    const unsubscribe = subscribeToPoseFrames(frame => {
      if (!detectionActiveRef.current) {
        return;
      }

      setLatestFrame(frame);
      setCapturedFrames(current => {
        const nextFrames = [...current, frame];
        if (nextFrames.length > 900) {
          return nextFrames.slice(nextFrames.length - 900);
        }
        return nextFrames;
      });
    });

    return () => {
      unsubscribe();
      resetFormCheckPoseRuntime();
    };
  }, []);

  async function requestPermission() {
    const status = await request(CAMERA_PERMISSION);
    setCameraPermission(status);
  }

  async function handlePermissionAction() {
    if (cameraPermission === RESULTS.BLOCKED) {
      await openSettings();
      return;
    }

    await requestPermission();
  }

  async function handleStartDetection() {
    if (!localConsentGranted) {
      setStatusMessage(undefined);
      setErrorMessage('Enable local form-check consent first.');
      return;
    }

    const isPermissionGranted = cameraPermission === RESULTS.GRANTED || cameraPermission === RESULTS.LIMITED;
    if (!isPermissionGranted) {
      setStatusMessage(undefined);
      setErrorMessage('Camera permission is required before recording.');
      return;
    }

    setErrorMessage(undefined);
    setStatusMessage('Recording started. Perform 2-5 smooth squat reps.');
    setResultSummary(null);
    setLatestFrame(null);
    setCapturedFrames([]);

    const startedAt = new Date().toISOString();
    setRecordingStartedAt(startedAt);
    setRecordingEndedAt(null);

    await startFormCheckDetection(MOVEMENT_TYPE);
    setIsDetecting(true);
  }

  async function handleStopDetection() {
    if (!isDetecting) {
      return;
    }

    setIsDetecting(false);
    const endedAt = new Date().toISOString();
    setRecordingEndedAt(endedAt);

    try {
      const summary = await stopFormCheckDetection(MOVEMENT_TYPE);
      setResultSummary(summary);
      setStatusMessage('Local analysis complete. Nothing leaves your device unless you upload.');
      setErrorMessage(undefined);
    } catch (error) {
      setStatusMessage(undefined);
      setErrorMessage(error instanceof Error ? error.message : 'Unable to complete local form check.');
    }
  }

  function handleUploadToCoach() {
    if (!resultSummary) {
      setStatusMessage(undefined);
      setErrorMessage('Run a form check before uploading.');
      return;
    }

    if (!isNetworkOnline()) {
      setStatusMessage(undefined);
      setErrorMessage('No network connection. Local results are saved on device only.');
      return;
    }

    uploadMutation.mutate();
  }

  if (!session) {
    return (
      <View style={styles.center} testID="form-check-no-session">
        <Text style={styles.error}>You must be logged in to use form check.</Text>
      </View>
    );
  }

  if (consentsQuery.isLoading) {
    return (
      <View style={styles.center} testID="form-check-loading">
        <ActivityIndicator size="large" color="#0f766e" />
      </View>
    );
  }

  if (consentsQuery.isError) {
    return (
      <View style={styles.center} testID="form-check-consent-error">
        <Text style={styles.error}>Unable to load privacy consents.</Text>
      </View>
    );
  }

  const isPermissionGranted = cameraPermission === RESULTS.GRANTED || cameraPermission === RESULTS.LIMITED;

  return (
    <ScrollView contentContainerStyle={styles.container} testID="form-check-screen">
      <Text style={styles.title}>Form Check v1</Text>
      <Text style={styles.subtitle}>
        On-device squat analysis using a MoveNet-compatible native pose pipeline.
      </Text>

      <Card testID="form-check-instructions-card">
        <Text style={styles.sectionTitle}>How it works</Text>
        <Text style={styles.helperText}>1. Place camera side-on with full body in frame.</Text>
        <Text style={styles.helperText}>2. Record 2-5 controlled squat reps.</Text>
        <Text style={styles.helperText}>3. Review ROM, knee tracking, and symmetry locally.</Text>
        <Text style={styles.helperText}>4. Upload to coach only if you explicitly choose Upload.</Text>
      </Card>

      {!localConsentGranted ? (
        <Card testID="form-check-consent-card">
          <Text style={styles.sectionTitle}>Consent Required</Text>
          <Text style={styles.helperText}>
            Form Check is local-only by default. Enable local form-check consent to run on-device pose
            estimation.
          </Text>
          <Button
            label="Enable Local Form Check"
            onPress={() => {
              grantLocalConsentMutation.mutate();
            }}
            loading={grantLocalConsentMutation.isPending}
            disabled={grantLocalConsentMutation.isPending}
            testID="form-check-enable-local-consent"
          />
          <Button
            label="Open Privacy Settings"
            variant="secondary"
            onPress={() => {
              navigation.navigate('PrivacySettings');
            }}
            testID="form-check-open-privacy-settings"
          />
        </Card>
      ) : cameraPermission === 'checking' ? (
        <Card testID="form-check-permission-loading-card">
          <ActivityIndicator size="small" color="#0f766e" testID="form-check-permission-loading" />
          <Text style={styles.helperText}>Checking camera permission...</Text>
        </Card>
      ) : !isPermissionGranted ? (
        <Card testID="form-check-permission-card">
          <Text style={styles.sectionTitle}>Camera Permission Required</Text>
          <Text style={styles.helperText}>
            {cameraPermission === RESULTS.BLOCKED
              ? 'Camera access is blocked. Open settings to allow access.'
              : 'Allow camera access to run on-device pose estimation.'}
          </Text>
          <Button
            label={cameraPermission === RESULTS.BLOCKED ? 'Open Settings' : 'Allow Camera'}
            onPress={() => {
              handlePermissionAction().catch(() => {
                setErrorMessage('Unable to update camera permission.');
              });
            }}
            testID="form-check-permission-button"
          />
        </Card>
      ) : (
        <Card testID="form-check-camera-card">
          <View style={styles.cameraWrapper}>
            <Camera style={styles.camera} testID="form-check-camera" />
          </View>
          <Text style={styles.helperText}>
            {isDetecting
              ? 'Recording now. Keep your full body in frame.'
              : 'Ready to record. Keep side profile visible.'}
          </Text>
          {latestFrame ? (
            <Text style={styles.liveAngles} testID="form-check-live-angles">
              L/R knee: {latestFrame.leftKneeDeg.toFixed(1)}° / {latestFrame.rightKneeDeg.toFixed(1)}°
            </Text>
          ) : null}
          <View style={styles.actionsRow}>
            {!isDetecting ? (
              <Button
                label="Start Recording"
                onPress={() => {
                  handleStartDetection().catch(error => {
                    setErrorMessage(error instanceof Error ? error.message : 'Unable to start detection.');
                  });
                }}
                testID="form-check-start-button"
              />
            ) : (
              <Button
                label="Stop & Analyze"
                variant="danger"
                onPress={() => {
                  handleStopDetection().catch(error => {
                    setErrorMessage(error instanceof Error ? error.message : 'Unable to stop detection.');
                  });
                }}
                testID="form-check-stop-button"
              />
            )}
          </View>
        </Card>
      )}

      {resultSummary ? (
        <Card testID="form-check-results-card">
          <Text style={styles.sectionTitle}>Result Summary</Text>
          <View style={styles.row}>
            <Text style={styles.metricLabel}>Overall score</Text>
            <Text style={styles.metricValue}>{resultSummary.overallScore}/100</Text>
          </View>
          <View style={styles.row}>
            <Text style={styles.metricLabel}>Range of motion</Text>
            <Text style={styles.metricValue}>
              {resultSummary.rangeOfMotionDegrees.toFixed(1)}° ({resultSummary.rangeOfMotionScore}/100)
            </Text>
          </View>
          <View style={styles.row}>
            <Text style={styles.metricLabel}>Knee tracking</Text>
            <Text style={styles.metricValue}>{resultSummary.kneeTrackingScore}/100</Text>
          </View>
          <View style={styles.row}>
            <Text style={styles.metricLabel}>Symmetry</Text>
            <Text style={styles.metricValue}>{resultSummary.symmetryScore}/100</Text>
          </View>
          <View style={styles.row}>
            <Text style={styles.metricLabel}>Reps detected</Text>
            <Text style={styles.metricValue}>{resultSummary.repetitionCount}</Text>
          </View>
          <Text style={styles.feedbackTitle}>Feedback</Text>
          {resultSummary.feedback.map(item => (
            <Text key={item} style={styles.feedbackItem}>
              • {item}
            </Text>
          ))}
          <Text style={styles.helperText}>
            Samples captured: {resultSummary.sampleCount} (live samples this run: {liveSummary.sampleCount})
          </Text>
        </Card>
      ) : null}

      {resultSummary && !uploadEntitled ? (
        <Card testID="form-check-upload-paywall">
          <Text style={styles.sectionTitle}>Upload to Coach (Pro)</Text>
          <Text style={styles.helperText}>
            You can run local form checks for free. Upload-to-coach requires a paid plan.
          </Text>
          <Button
            label="Upgrade"
            variant="secondary"
            onPress={() => {
              navigation.navigate('Paywall', {
                feature: 'form_check_upload',
              });
            }}
            testID="form-check-upload-upgrade-button"
          />
        </Card>
      ) : null}

      {resultSummary && uploadEntitled && !uploadConsentGranted ? (
        <Card testID="form-check-upload-consent-card">
          <Text style={styles.sectionTitle}>Upload Consent Required</Text>
          <Text style={styles.helperText}>
            Upload is optional. Enable upload consent only if you want this result sent to your coach.
          </Text>
          <Button
            label="Enable Upload Consent"
            onPress={() => {
              grantUploadConsentMutation.mutate();
            }}
            loading={grantUploadConsentMutation.isPending}
            disabled={grantUploadConsentMutation.isPending}
            testID="form-check-enable-upload-consent"
          />
        </Card>
      ) : null}

      {resultSummary && uploadEntitled && uploadConsentGranted ? (
        <Card testID="form-check-upload-card">
          <Text style={styles.sectionTitle}>Upload to Coach</Text>
          <Text style={styles.helperText}>
            Upload only happens when you tap the button below. No automatic or background uploads.
          </Text>
          <Button
            label="Upload to Coach"
            onPress={handleUploadToCoach}
            loading={uploadMutation.isPending}
            disabled={uploadMutation.isPending}
            testID="form-check-upload-button"
          />
        </Card>
      ) : null}

      {errorMessage ? <Text style={styles.error}>{errorMessage}</Text> : null}
      {statusMessage ? <Text style={styles.success}>{statusMessage}</Text> : null}
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  container: {
    padding: 16,
    gap: 12,
    backgroundColor: '#f8fafc',
  },
  center: {
    flex: 1,
    alignItems: 'center',
    justifyContent: 'center',
    padding: 16,
    backgroundColor: '#f8fafc',
  },
  title: {
    fontSize: 24,
    fontWeight: '700',
    color: '#0f172a',
  },
  subtitle: {
    color: '#475569',
  },
  sectionTitle: {
    fontSize: 18,
    fontWeight: '700',
    color: '#0f172a',
  },
  helperText: {
    color: '#475569',
    fontSize: 13,
  },
  cameraWrapper: {
    borderRadius: 12,
    overflow: 'hidden',
    borderWidth: 1,
    borderColor: '#cbd5e1',
  },
  camera: {
    width: '100%',
    minHeight: 280,
  },
  liveAngles: {
    color: '#0f172a',
    fontSize: 13,
    fontWeight: '600',
  },
  actionsRow: {
    marginTop: 8,
  },
  row: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    gap: 10,
  },
  metricLabel: {
    color: '#334155',
    fontSize: 14,
  },
  metricValue: {
    color: '#0f172a',
    fontWeight: '700',
    fontSize: 14,
  },
  feedbackTitle: {
    color: '#0f172a',
    fontWeight: '700',
    marginTop: 8,
  },
  feedbackItem: {
    color: '#334155',
    fontSize: 13,
  },
  error: {
    color: '#b91c1c',
    fontSize: 13,
  },
  success: {
    color: '#0f766e',
    fontSize: 13,
  },
});
