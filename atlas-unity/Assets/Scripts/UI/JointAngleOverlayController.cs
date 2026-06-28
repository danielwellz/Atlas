using System;
using System.Collections.Generic;
using UnityEngine;
using UnityEngine.UI;

public sealed class JointAngleOverlayController : MonoBehaviour
{
    [Serializable]
    public sealed class JointAngleWidget
    {
        public string joint;
        public Transform proximalBone;
        public Transform jointBone;
        public Transform distalBone;
        public Text valueText;
        public Image arcImage;
    }

    private const float MinimumVectorMagnitude = 0.0001f;

    private readonly Dictionary<string, JointAngleMetadata> _anglesByJoint =
        new Dictionary<string, JointAngleMetadata>(StringComparer.OrdinalIgnoreCase);

    [SerializeField] private GameObject overlayRoot;
    [SerializeField] private JointAngleWidget[] widgets;
    [SerializeField] private bool overlayEnabled = true;

    private void Awake()
    {
        ApplyOverlayVisibility();
    }

    public void Apply(JointAngleMetadata[] jointAngles)
    {
        _anglesByJoint.Clear();

        if (jointAngles != null)
        {
            foreach (JointAngleMetadata angle in jointAngles)
            {
                if (angle == null || string.IsNullOrWhiteSpace(angle.joint))
                {
                    continue;
                }

                _anglesByJoint[angle.joint] = angle;
            }
        }

        UpdateWidgets();
    }

    public void SetOverlayEnabled(bool enabled)
    {
        overlayEnabled = enabled;
        ApplyOverlayVisibility();
    }

    private void LateUpdate()
    {
        UpdateWidgets();
    }

    private void UpdateWidgets()
    {
        if (!overlayEnabled)
        {
            return;
        }

        if (widgets == null)
        {
            return;
        }

        foreach (JointAngleWidget widget in widgets)
        {
            if (widget == null || string.IsNullOrWhiteSpace(widget.joint))
            {
                continue;
            }

            if (!_anglesByJoint.TryGetValue(widget.joint, out JointAngleMetadata angle))
            {
                SetWidget(widget, widget.joint, 0f, 0f, 0f, "deg", false);
                continue;
            }

            float displayedAngle = angle.targetDegrees;
            if (TryMeasureJointAngle(widget, out float measuredAngle))
            {
                displayedAngle = measuredAngle;
            }

            SetWidget(widget, angle.joint, angle.minDegrees, angle.maxDegrees, displayedAngle, angle.unit, true);
        }
    }

    private void ApplyOverlayVisibility()
    {
        if (overlayRoot != null)
        {
            overlayRoot.SetActive(overlayEnabled);
            return;
        }

        if (widgets == null)
        {
            return;
        }

        foreach (JointAngleWidget widget in widgets)
        {
            if (widget == null)
            {
                continue;
            }

            if (widget.valueText != null)
            {
                widget.valueText.enabled = overlayEnabled;
            }

            if (widget.arcImage != null)
            {
                widget.arcImage.enabled = overlayEnabled;
            }
        }
    }

    private static bool TryMeasureJointAngle(JointAngleWidget widget, out float measuredAngle)
    {
        measuredAngle = 0f;
        if (widget == null || widget.proximalBone == null || widget.jointBone == null || widget.distalBone == null)
        {
            return false;
        }

        Vector3 proximalVector = widget.proximalBone.position - widget.jointBone.position;
        Vector3 distalVector = widget.distalBone.position - widget.jointBone.position;
        if (proximalVector.sqrMagnitude < MinimumVectorMagnitude || distalVector.sqrMagnitude < MinimumVectorMagnitude)
        {
            return false;
        }

        measuredAngle = Vector3.Angle(proximalVector, distalVector);
        return true;
    }

    private static void SetWidget(
        JointAngleWidget widget,
        string joint,
        float minDegrees,
        float maxDegrees,
        float targetDegrees,
        string unit,
        bool hasData)
    {
        if (widget.valueText != null)
        {
            if (!hasData)
            {
                widget.valueText.text = joint + ": --";
            }
            else
            {
                widget.valueText.text = string.Format(
                    "{0}: {1:0.#}{4} ({2:0.#}-{3:0.#}{4})",
                    joint,
                    targetDegrees,
                    minDegrees,
                    maxDegrees,
                    string.IsNullOrWhiteSpace(unit) ? "" : unit
                );
            }
        }

        if (widget.arcImage != null)
        {
            if (!hasData)
            {
                widget.arcImage.fillAmount = 0f;
                return;
            }

            float denominator = Mathf.Max(maxDegrees - minDegrees, 0.0001f);
            float normalized = Mathf.Clamp01((targetDegrees - minDegrees) / denominator);
            widget.arcImage.fillAmount = normalized;
        }
    }
}
