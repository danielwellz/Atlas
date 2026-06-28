using System;
using System.Collections.Generic;
using UnityEngine;

public sealed class BiomechanicsPlaybackController : MonoBehaviour
{
    [Serializable]
    public sealed class MuscleGroupRendererBinding
    {
        public string muscleGroup;
        public Renderer[] renderers;
    }

    private static readonly int HighlightColorID = Shader.PropertyToID("_HighlightColor");
    private static readonly int HighlightIntensityID = Shader.PropertyToID("_HighlightIntensity");

    private readonly MaterialPropertyBlock _propertyBlock = new MaterialPropertyBlock();

    [SerializeField] private Animator animator;
    [SerializeField] private string animatorLayerName = "Base Layer";
    [SerializeField] private string fallbackStateName = "Idle";
    [SerializeField] private MuscleGroupRendererBinding[] muscleRenderBindings;
    [SerializeField] private Renderer[] muscleLayerRenderers;
    [SerializeField] private Renderer[] skeletonLayerRenderers;
    [SerializeField] private JointAngleOverlayController jointAngleOverlay;

    public static BiomechanicsPlaybackController Instance { get; private set; }

    private ExerciseBiomechanicsMetadata _activeMetadata;
    private bool _showMuscles = true;
    private bool _showSkeleton = true;

    private void Awake()
    {
        Instance = this;
        ApplyLayerVisibility();
    }

    private void OnDestroy()
    {
        if (Instance == this)
        {
            Instance = null;
        }
    }

    public void PreviewFromJSON(string json)
    {
        LoadExerciseBiomechanicsFromJSON(json);
    }

    public void LoadExerciseBiomechanicsFromJSON(string json)
    {
        if (string.IsNullOrWhiteSpace(json))
        {
            return;
        }

        ExerciseBiomechanicsMetadata metadata;
        try
        {
            metadata = JsonUtility.FromJson<ExerciseBiomechanicsMetadata>(json);
        }
        catch (Exception)
        {
            return;
        }

        if (metadata == null)
        {
            return;
        }

        LoadExerciseBiomechanics(metadata);
    }

    public void Preview(ExerciseBiomechanicsMetadata metadata)
    {
        LoadExerciseBiomechanics(metadata);
    }

    public void LoadExerciseBiomechanics(ExerciseBiomechanicsMetadata metadata)
    {
        if (metadata == null)
        {
            return;
        }

        _activeMetadata = metadata;
        PlayAnimation(metadata.animationAssetKey);
        ApplyMuscleHighlights(metadata.muscleHighlights);

        if (jointAngleOverlay != null)
        {
            jointAngleOverlay.Apply(metadata.jointAngles);
        }
    }

    public void SetHighlightMuscles(MuscleHighlightMetadata[] highlights, string[] muscleGroups)
    {
        MuscleHighlightMetadata[] normalizedHighlights = NormalizeHighlights(highlights, muscleGroups);
        ApplyMuscleHighlights(normalizedHighlights);
    }

    public void SetLayerVisibility(bool showSkeleton, bool showMuscles)
    {
        _showSkeleton = showSkeleton;
        _showMuscles = showMuscles;
        ApplyLayerVisibility();
    }

    public void SetJointAngleOverlay(bool enabled, JointAngleMetadata[] jointAngles)
    {
        if (jointAngleOverlay == null)
        {
            return;
        }

        jointAngleOverlay.SetOverlayEnabled(enabled);

        if (jointAngles != null && jointAngles.Length > 0)
        {
            jointAngleOverlay.Apply(jointAngles);
            return;
        }

        if (_activeMetadata != null)
        {
            jointAngleOverlay.Apply(_activeMetadata.jointAngles);
        }
    }

    private void PlayAnimation(string animationAssetKey)
    {
        if (animator == null)
        {
            return;
        }

        int layerIndex = animator.GetLayerIndex(animatorLayerName);
        if (layerIndex < 0)
        {
            layerIndex = 0;
        }

        foreach (string stateCandidate in DeriveStateCandidates(animationAssetKey))
        {
            if (TryCrossFadeToState(stateCandidate, layerIndex))
            {
                return;
            }
        }

        if (!string.IsNullOrWhiteSpace(fallbackStateName))
        {
            TryCrossFadeToState(fallbackStateName, layerIndex);
        }
    }

    private bool TryCrossFadeToState(string stateName, int layerIndex)
    {
        if (string.IsNullOrWhiteSpace(stateName))
        {
            return false;
        }

        int stateHash = Animator.StringToHash(stateName);
        if (!animator.HasState(layerIndex, stateHash))
        {
            return false;
        }

        animator.CrossFadeInFixedTime(stateHash, 0.15f, layerIndex);
        return true;
    }

    private static string[] DeriveStateCandidates(string animationAssetKey)
    {
        if (string.IsNullOrWhiteSpace(animationAssetKey))
        {
            return Array.Empty<string>();
        }

        string[] pathParts = animationAssetKey.Split('/');
        string leaf = pathParts[pathParts.Length - 1];
        int extensionIndex = leaf.LastIndexOf('.');
        if (extensionIndex > 0)
        {
            leaf = leaf.Substring(0, extensionIndex);
        }

        string parent = pathParts.Length > 1 ? pathParts[pathParts.Length - 2] : string.Empty;
        HashSet<string> deduplicated = new HashSet<string>(StringComparer.OrdinalIgnoreCase);
        List<string> ordered = new List<string>(3);

        AddStateCandidate(deduplicated, ordered, parent + "_" + leaf);
        AddStateCandidate(deduplicated, ordered, parent);
        AddStateCandidate(deduplicated, ordered, leaf);

        return ordered.ToArray();
    }

    private static void AddStateCandidate(HashSet<string> deduplicated, List<string> ordered, string value)
    {
        if (string.IsNullOrWhiteSpace(value))
        {
            return;
        }

        string normalized = value.Trim().Replace('-', '_').Replace(' ', '_');
        if (string.IsNullOrWhiteSpace(normalized))
        {
            return;
        }

        if (deduplicated.Add(normalized))
        {
            ordered.Add(normalized);
        }
    }

    private void ApplyMuscleHighlights(MuscleHighlightMetadata[] highlights)
    {
        Dictionary<string, MuscleHighlightMetadata> highlightByGroup =
            new Dictionary<string, MuscleHighlightMetadata>(StringComparer.OrdinalIgnoreCase);

        if (highlights != null)
        {
            foreach (MuscleHighlightMetadata highlight in highlights)
            {
                if (highlight == null || string.IsNullOrWhiteSpace(highlight.muscleGroup))
                {
                    continue;
                }

                highlightByGroup[highlight.muscleGroup] = highlight;
            }
        }

        if (muscleRenderBindings == null)
        {
            return;
        }

        foreach (MuscleGroupRendererBinding binding in muscleRenderBindings)
        {
            if (binding == null || binding.renderers == null)
            {
                continue;
            }

            bool hasHighlight = binding.muscleGroup != null &&
                highlightByGroup.TryGetValue(binding.muscleGroup, out MuscleHighlightMetadata highlight);

            float intensity = hasHighlight ? Mathf.Clamp01(highlight.activationLevel) : 0f;
            Color color = hasHighlight && ColorUtility.TryParseHtmlString(highlight.colorHex, out Color parsed)
                ? parsed
                : new Color(1f, 0.42f, 0.2f, 1f);

            foreach (Renderer targetRenderer in binding.renderers)
            {
                if (targetRenderer == null)
                {
                    continue;
                }

                targetRenderer.GetPropertyBlock(_propertyBlock);
                _propertyBlock.SetFloat(HighlightIntensityID, intensity);
                _propertyBlock.SetColor(HighlightColorID, color);
                targetRenderer.SetPropertyBlock(_propertyBlock);
            }
        }
    }

    private MuscleHighlightMetadata[] NormalizeHighlights(MuscleHighlightMetadata[] highlights, string[] muscleGroups)
    {
        if (highlights != null && highlights.Length > 0)
        {
            return highlights;
        }

        if (muscleGroups == null || muscleGroups.Length == 0)
        {
            return _activeMetadata != null && _activeMetadata.muscleHighlights != null
                ? _activeMetadata.muscleHighlights
                : Array.Empty<MuscleHighlightMetadata>();
        }

        Dictionary<string, MuscleHighlightMetadata> sourceByGroup =
            new Dictionary<string, MuscleHighlightMetadata>(StringComparer.OrdinalIgnoreCase);

        if (_activeMetadata != null && _activeMetadata.muscleHighlights != null)
        {
            foreach (MuscleHighlightMetadata sourceHighlight in _activeMetadata.muscleHighlights)
            {
                if (sourceHighlight == null || string.IsNullOrWhiteSpace(sourceHighlight.muscleGroup))
                {
                    continue;
                }

                sourceByGroup[sourceHighlight.muscleGroup] = sourceHighlight;
            }
        }

        List<MuscleHighlightMetadata> normalized = new List<MuscleHighlightMetadata>(muscleGroups.Length);
        foreach (string muscleGroup in muscleGroups)
        {
            if (string.IsNullOrWhiteSpace(muscleGroup))
            {
                continue;
            }

            sourceByGroup.TryGetValue(muscleGroup, out MuscleHighlightMetadata source);
            normalized.Add(new MuscleHighlightMetadata
            {
                muscleGroup = muscleGroup,
                activationLevel = source != null ? source.activationLevel : 1f,
                role = source != null ? source.role : "primary",
                colorHex = source != null ? source.colorHex : "#FF6B35",
            });
        }

        return normalized.ToArray();
    }

    private void ApplyLayerVisibility()
    {
        ApplyRendererVisibility(GetMuscleLayerRenderers(), _showMuscles);
        ApplyRendererVisibility(skeletonLayerRenderers, _showSkeleton);
    }

    private Renderer[] GetMuscleLayerRenderers()
    {
        if (muscleLayerRenderers != null && muscleLayerRenderers.Length > 0)
        {
            return muscleLayerRenderers;
        }

        if (muscleRenderBindings == null || muscleRenderBindings.Length == 0)
        {
            return Array.Empty<Renderer>();
        }

        HashSet<Renderer> deduplicated = new HashSet<Renderer>();
        List<Renderer> collected = new List<Renderer>();

        foreach (MuscleGroupRendererBinding binding in muscleRenderBindings)
        {
            if (binding == null || binding.renderers == null)
            {
                continue;
            }

            foreach (Renderer targetRenderer in binding.renderers)
            {
                if (targetRenderer == null || !deduplicated.Add(targetRenderer))
                {
                    continue;
                }

                collected.Add(targetRenderer);
            }
        }

        return collected.ToArray();
    }

    private static void ApplyRendererVisibility(Renderer[] renderers, bool visible)
    {
        if (renderers == null)
        {
            return;
        }

        foreach (Renderer targetRenderer in renderers)
        {
            if (targetRenderer == null)
            {
                continue;
            }

            targetRenderer.enabled = visible;
        }
    }
}
