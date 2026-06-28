Shader "Atlas/MuscleHighlight"
{
    Properties
    {
        _MainTex ("Main Texture", 2D) = "white" {}
        _BaseColor ("Base Color", Color) = (1,1,1,1)
        _HighlightColor ("Highlight Color", Color) = (1,0.42,0.2,1)
        _HighlightIntensity ("Highlight Intensity", Range(0,1)) = 0
    }

    SubShader
    {
        Tags { "RenderType" = "Opaque" }
        LOD 100

        Pass
        {
            CGPROGRAM
            #pragma vertex vert
            #pragma fragment frag
            #include "UnityCG.cginc"

            struct appdata
            {
                float4 vertex : POSITION;
                float2 uv : TEXCOORD0;
            };

            struct v2f
            {
                float2 uv : TEXCOORD0;
                float4 vertex : SV_POSITION;
            };

            sampler2D _MainTex;
            float4 _MainTex_ST;
            fixed4 _BaseColor;
            fixed4 _HighlightColor;
            float _HighlightIntensity;

            v2f vert(appdata v)
            {
                v2f o;
                o.vertex = UnityObjectToClipPos(v.vertex);
                o.uv = TRANSFORM_TEX(v.uv, _MainTex);
                return o;
            }

            fixed4 frag(v2f i) : SV_Target
            {
                fixed4 sampled = tex2D(_MainTex, i.uv) * _BaseColor;
                fixed3 highlighted = lerp(sampled.rgb, _HighlightColor.rgb, saturate(_HighlightIntensity));
                return fixed4(highlighted, sampled.a);
            }
            ENDCG
        }
    }
}
