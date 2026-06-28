-- Idempotent local seed data for Atlas API.
-- Demo user:
--   email: demo@atlas.local
--   password: atlas1234

INSERT INTO users (id, email, password_hash, created_at)
VALUES (
    '00000000-0000-0000-0000-000000000001',
    'demo@atlas.local',
    '$2y$10$eEGZKS1RlOMYrsKQeuvfLeC0YpuIfkCQaPyt5lyuqq.hKU2/gzDm2',
    NOW()
)
ON CONFLICT (email) DO NOTHING;

INSERT INTO consents (id, user_id, consent_type, granted_at, revoked_at, metadata_json)
VALUES (
    '10000000-0000-0000-0000-000000000001',
    '00000000-0000-0000-0000-000000000001',
    'progress_photos',
    NOW(),
    NULL,
    '{"source":"seed"}'::jsonb
)
ON CONFLICT (user_id, consent_type)
DO UPDATE SET
    granted_at = EXCLUDED.granted_at,
    revoked_at = EXCLUDED.revoked_at,
    metadata_json = EXCLUDED.metadata_json;

INSERT INTO user_profiles (
    user_id,
    display_name,
    sex,
    height_cm,
    weight_kg,
    experience_level,
    created_at,
    updated_at
)
VALUES (
    '00000000-0000-0000-0000-000000000001',
    'Atlas Demo',
    'male',
    180,
    82.5,
    'intermediate',
    NOW(),
    NOW()
)
ON CONFLICT (user_id)
DO UPDATE SET
    display_name = EXCLUDED.display_name,
    sex = EXCLUDED.sex,
    height_cm = EXCLUDED.height_cm,
    weight_kg = EXCLUDED.weight_kg,
    experience_level = EXCLUDED.experience_level,
    updated_at = NOW();

INSERT INTO user_goals (
    user_id,
    primary_goal,
    secondary_goal,
    days_per_week,
    session_duration_minutes,
    equipment_access_json,
    constraints_json,
    created_at,
    updated_at
)
VALUES (
    '00000000-0000-0000-0000-000000000001',
    'build_strength',
    'improve_mobility',
    4,
    60,
    '["barbell","dumbbell","bench"]'::jsonb,
    '{"knee_pain":false}'::jsonb,
    NOW(),
    NOW()
)
ON CONFLICT (user_id)
DO UPDATE SET
    primary_goal = EXCLUDED.primary_goal,
    secondary_goal = EXCLUDED.secondary_goal,
    days_per_week = EXCLUDED.days_per_week,
    session_duration_minutes = EXCLUDED.session_duration_minutes,
    equipment_access_json = EXCLUDED.equipment_access_json,
    constraints_json = EXCLUDED.constraints_json,
    updated_at = NOW();

-- Minimal exercise set required for seeded program templates.
INSERT INTO exercises (id, slug, name, primary_muscle_group, secondary_muscles_json, movement_pattern, equipment_json, difficulty, description, created_at)
VALUES
    ('20000000-0000-0000-0000-000000000001', 'back-squat', 'Back Squat', 'quads', '["glutes","core"]'::jsonb, 'squat', '["barbell","rack"]'::jsonb, 'intermediate', 'Barbell squat pattern.', NOW()),
    ('20000000-0000-0000-0000-000000000002', 'bench-press', 'Bench Press', 'chest', '["triceps","front_delts"]'::jsonb, 'push', '["barbell","bench"]'::jsonb, 'intermediate', 'Barbell horizontal press.', NOW()),
    ('20000000-0000-0000-0000-000000000003', 'barbell-row', 'Barbell Row', 'upper_back', '["lats","biceps"]'::jsonb, 'pull', '["barbell"]'::jsonb, 'intermediate', 'Bent-over horizontal pull.', NOW()),
    ('20000000-0000-0000-0000-000000000004', 'overhead-press', 'Overhead Press', 'shoulders', '["triceps","core"]'::jsonb, 'push', '["barbell"]'::jsonb, 'intermediate', 'Standing vertical press.', NOW()),
    ('20000000-0000-0000-0000-000000000005', 'conventional-deadlift', 'Conventional Deadlift', 'glutes', '["hamstrings","erectors"]'::jsonb, 'hinge', '["barbell"]'::jsonb, 'advanced', 'Conventional barbell deadlift.', NOW()),
    ('20000000-0000-0000-0000-000000000006', 'romanian-deadlift', 'Romanian Deadlift', 'hamstrings', '["glutes","erectors"]'::jsonb, 'hinge', '["barbell","dumbbell"]'::jsonb, 'intermediate', 'Loaded hip hinge with stretch focus.', NOW())
ON CONFLICT (slug) DO NOTHING;

INSERT INTO exercise_biomech_assets (id, exercise_id, animation_asset_key, rig_version, metadata_json, created_at, updated_at)
SELECT
    seeded.id,
    e.id,
    seeded.animation_asset_key,
    seeded.rig_version,
    seeded.metadata_json,
    NOW(),
    NOW()
FROM (
    VALUES
        (
            '26000000-0000-0000-0000-000000000001'::uuid,
            'back-squat',
            'biomechanics/back-squat/clip_v1.fbx',
            'atlas-humanoid-v1',
            '{"muscleHighlights":[{"muscleGroup":"quads","activationLevel":1.0,"role":"primary","colorHex":"#FF6B35"},{"muscleGroup":"glutes","activationLevel":0.82,"role":"secondary","colorHex":"#F97316"},{"muscleGroup":"core","activationLevel":0.55,"role":"stabilizer","colorHex":"#FB923C"}],"jointAngles":[{"joint":"knee","minDegrees":70,"maxDegrees":175,"targetDegrees":95,"unit":"deg"},{"joint":"hip","minDegrees":55,"maxDegrees":170,"targetDegrees":100,"unit":"deg"},{"joint":"ankle","minDegrees":60,"maxDegrees":120,"targetDegrees":88,"unit":"deg"}]}'::jsonb
        ),
        (
            '26000000-0000-0000-0000-000000000002'::uuid,
            'bench-press',
            'biomechanics/bench-press/clip_v1.fbx',
            'atlas-humanoid-v1',
            '{"muscleHighlights":[{"muscleGroup":"chest","activationLevel":1.0,"role":"primary","colorHex":"#F43F5E"},{"muscleGroup":"triceps","activationLevel":0.86,"role":"secondary","colorHex":"#FB7185"},{"muscleGroup":"shoulders","activationLevel":0.68,"role":"secondary","colorHex":"#FDA4AF"}],"jointAngles":[{"joint":"elbow","minDegrees":65,"maxDegrees":178,"targetDegrees":92,"unit":"deg"},{"joint":"shoulder","minDegrees":20,"maxDegrees":130,"targetDegrees":76,"unit":"deg"}]}'::jsonb
        ),
        (
            '26000000-0000-0000-0000-000000000003'::uuid,
            'barbell-row',
            'biomechanics/barbell-row/clip_v1.fbx',
            'atlas-humanoid-v1',
            '{"muscleHighlights":[{"muscleGroup":"upper_back","activationLevel":1.0,"role":"primary","colorHex":"#38BDF8"},{"muscleGroup":"lats","activationLevel":0.83,"role":"secondary","colorHex":"#7DD3FC"},{"muscleGroup":"biceps","activationLevel":0.61,"role":"secondary","colorHex":"#BAE6FD"}],"jointAngles":[{"joint":"elbow","minDegrees":50,"maxDegrees":165,"targetDegrees":85,"unit":"deg"},{"joint":"hip","minDegrees":35,"maxDegrees":115,"targetDegrees":62,"unit":"deg"}]}'::jsonb
        )
) AS seeded(id, exercise_slug, animation_asset_key, rig_version, metadata_json)
JOIN exercises e ON e.slug = seeded.exercise_slug
ON CONFLICT (exercise_id)
DO UPDATE SET
    animation_asset_key = EXCLUDED.animation_asset_key,
    rig_version = EXCLUDED.rig_version,
    metadata_json = EXCLUDED.metadata_json,
    updated_at = NOW();

INSERT INTO exercise_biomech_asset_muscle_groups (biomech_asset_id, muscle_group_slug, activation_level, role)
VALUES
    ('26000000-0000-0000-0000-000000000001'::uuid, 'quads', 1.00, 'primary'),
    ('26000000-0000-0000-0000-000000000001'::uuid, 'glutes', 0.82, 'secondary'),
    ('26000000-0000-0000-0000-000000000001'::uuid, 'core', 0.55, 'stabilizer'),
    ('26000000-0000-0000-0000-000000000002'::uuid, 'chest', 1.00, 'primary'),
    ('26000000-0000-0000-0000-000000000002'::uuid, 'triceps', 0.86, 'secondary'),
    ('26000000-0000-0000-0000-000000000002'::uuid, 'shoulders', 0.68, 'secondary'),
    ('26000000-0000-0000-0000-000000000003'::uuid, 'upper_back', 1.00, 'primary'),
    ('26000000-0000-0000-0000-000000000003'::uuid, 'lats', 0.83, 'secondary'),
    ('26000000-0000-0000-0000-000000000003'::uuid, 'biceps', 0.61, 'secondary')
ON CONFLICT (biomech_asset_id, muscle_group_slug, role)
DO UPDATE SET
    activation_level = EXCLUDED.activation_level;

-- Program templates.
INSERT INTO programs (id, slug, name, description, goal_tags_json, level, weeks_length, created_at)
VALUES
    (
        '40000000-0000-0000-0000-000000000001',
        'hypertrophy-foundations-3-days',
        'Hypertrophy Foundations (3 days/week)',
        'Three-day full-body split focused on muscle growth fundamentals.',
        '["hypertrophy","muscle_gain","foundations"]'::jsonb,
        'beginner',
        8,
        NOW()
    ),
    (
        '40000000-0000-0000-0000-000000000002',
        'strength-foundations-3-days',
        'Strength Foundations (3 days/week)',
        'Three-day barbell-focused split to build baseline strength capacity.',
        '["strength","barbell","foundations"]'::jsonb,
        'intermediate',
        8,
        NOW()
    )
ON CONFLICT (slug)
DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    goal_tags_json = EXCLUDED.goal_tags_json,
    level = EXCLUDED.level,
    weeks_length = EXCLUDED.weeks_length;

INSERT INTO program_weeks (id, program_id, week_index, created_at)
SELECT
    v.id,
    p.id,
    v.week_index,
    NOW()
FROM (
    VALUES
        ('41000000-0000-0000-0000-000000000001'::uuid, 'hypertrophy-foundations-3-days', 1),
        ('41000000-0000-0000-0000-000000000002'::uuid, 'strength-foundations-3-days', 1)
) AS v(id, program_slug, week_index)
JOIN programs p ON p.slug = v.program_slug
ON CONFLICT (program_id, week_index) DO NOTHING;

INSERT INTO program_sessions (id, program_week_id, day_of_week, name, created_at)
SELECT
    v.id,
    pw.id,
    v.day_of_week,
    v.name,
    NOW()
FROM (
    VALUES
        ('42000000-0000-0000-0000-000000000001'::uuid, 'hypertrophy-foundations-3-days', 1, 1, 'Day 1 - Push Focus'),
        ('42000000-0000-0000-0000-000000000002'::uuid, 'hypertrophy-foundations-3-days', 1, 3, 'Day 2 - Lower Focus'),
        ('42000000-0000-0000-0000-000000000003'::uuid, 'hypertrophy-foundations-3-days', 1, 5, 'Day 3 - Pull Focus'),
        ('42000000-0000-0000-0000-000000000004'::uuid, 'strength-foundations-3-days', 1, 1, 'Day 1 - Squat + Press'),
        ('42000000-0000-0000-0000-000000000005'::uuid, 'strength-foundations-3-days', 1, 3, 'Day 2 - Deadlift + Pull'),
        ('42000000-0000-0000-0000-000000000006'::uuid, 'strength-foundations-3-days', 1, 5, 'Day 3 - Volume Bench + Row')
) AS v(id, program_slug, week_index, day_of_week, name)
JOIN programs p ON p.slug = v.program_slug
JOIN program_weeks pw ON pw.program_id = p.id AND pw.week_index = v.week_index
ON CONFLICT (program_week_id, day_of_week)
DO UPDATE SET
    name = EXCLUDED.name;

INSERT INTO program_session_exercises (id, program_session_id, exercise_id, prescription_json, order_index)
SELECT
    v.id,
    ps.id,
    e.id,
    v.prescription_json,
    v.order_index
FROM (
    VALUES
        ('43000000-0000-0000-0000-000000000001'::uuid, 'hypertrophy-foundations-3-days', 1, 1, 'bench-press', '{"sets":4,"reps_range":"8-12","rest_seconds":120,"rpe_target":7.5}'::jsonb, 1),
        ('43000000-0000-0000-0000-000000000002'::uuid, 'hypertrophy-foundations-3-days', 1, 1, 'overhead-press', '{"sets":3,"reps_range":"8-10","rest_seconds":90}'::jsonb, 2),
        ('43000000-0000-0000-0000-000000000003'::uuid, 'hypertrophy-foundations-3-days', 1, 3, 'back-squat', '{"sets":4,"reps_range":"8-10","rest_seconds":150,"rpe_target":8}'::jsonb, 1),
        ('43000000-0000-0000-0000-000000000004'::uuid, 'hypertrophy-foundations-3-days', 1, 3, 'romanian-deadlift', '{"sets":3,"reps_range":"10-12","rest_seconds":120}'::jsonb, 2),
        ('43000000-0000-0000-0000-000000000005'::uuid, 'hypertrophy-foundations-3-days', 1, 5, 'barbell-row', '{"sets":4,"reps_range":"8-12","rest_seconds":120}'::jsonb, 1),
        ('43000000-0000-0000-0000-000000000006'::uuid, 'hypertrophy-foundations-3-days', 1, 5, 'bench-press', '{"sets":3,"reps_range":"10-12","rest_seconds":90}'::jsonb, 2),
        ('43000000-0000-0000-0000-000000000007'::uuid, 'strength-foundations-3-days', 1, 1, 'back-squat', '{"sets":5,"reps_range":"3-5","rest_seconds":180,"rpe_target":8}'::jsonb, 1),
        ('43000000-0000-0000-0000-000000000008'::uuid, 'strength-foundations-3-days', 1, 1, 'overhead-press', '{"sets":5,"reps_range":"3-5","rest_seconds":150,"rpe_target":8}'::jsonb, 2),
        ('43000000-0000-0000-0000-000000000009'::uuid, 'strength-foundations-3-days', 1, 3, 'conventional-deadlift', '{"sets":5,"reps_range":"2-4","rest_seconds":210,"rpe_target":8.5}'::jsonb, 1),
        ('43000000-0000-0000-0000-000000000010'::uuid, 'strength-foundations-3-days', 1, 3, 'barbell-row', '{"sets":4,"reps_range":"5-8","rest_seconds":120}'::jsonb, 2),
        ('43000000-0000-0000-0000-000000000011'::uuid, 'strength-foundations-3-days', 1, 5, 'bench-press', '{"sets":5,"reps_range":"3-5","rest_seconds":180,"rpe_target":8}'::jsonb, 1),
        ('43000000-0000-0000-0000-000000000012'::uuid, 'strength-foundations-3-days', 1, 5, 'romanian-deadlift', '{"sets":4,"reps_range":"6-8","rest_seconds":150}'::jsonb, 2)
) AS v(id, program_slug, week_index, day_of_week, exercise_slug, prescription_json, order_index)
JOIN programs p ON p.slug = v.program_slug
JOIN program_weeks pw ON pw.program_id = p.id AND pw.week_index = v.week_index
JOIN program_sessions ps ON ps.program_week_id = pw.id AND ps.day_of_week = v.day_of_week
JOIN exercises e ON e.slug = v.exercise_slug
ON CONFLICT (program_session_id, order_index)
DO UPDATE SET
    exercise_id = EXCLUDED.exercise_id,
    prescription_json = EXCLUDED.prescription_json;

-- Seeded recipes for early meal-plan generation.
INSERT INTO recipes (
    id,
    slug,
    name,
    meal_type,
    description,
    servings,
    calories_kcal,
    protein_g,
    carbs_g,
    fat_g,
    ingredients_json,
    instructions_json,
    created_at,
    updated_at
)
VALUES
    (
        '50000000-0000-0000-0000-000000000001'::uuid,
        'overnight-oats-protein',
        'Protein Overnight Oats',
        'breakfast',
        'Oats with yogurt and berries.',
        1,
        420,
        30,
        56,
        12,
        '[{"name":"rolled oats","quantity":60,"unit":"g","category":"grains"},{"name":"greek yogurt","quantity":170,"unit":"g","category":"dairy"},{"name":"berries","quantity":100,"unit":"g","category":"produce"}]'::jsonb,
        '["Combine ingredients in a jar.","Chill overnight."]'::jsonb,
        NOW(),
        NOW()
    ),
    (
        '50000000-0000-0000-0000-000000000002'::uuid,
        'egg-veggie-scramble',
        'Egg Veggie Scramble',
        'breakfast',
        'Eggs scrambled with spinach.',
        1,
        390,
        28,
        24,
        18,
        '[{"name":"eggs","quantity":3,"unit":"item","category":"protein"},{"name":"spinach","quantity":80,"unit":"g","category":"produce"},{"name":"whole grain toast","quantity":1,"unit":"slice","category":"grains"}]'::jsonb,
        '["Scramble eggs and spinach.","Serve with toast."]'::jsonb,
        NOW(),
        NOW()
    ),
    (
        '50000000-0000-0000-0000-000000000003'::uuid,
        'chicken-rice-bowl',
        'Chicken Rice Bowl',
        'lunch',
        'Chicken breast with rice and vegetables.',
        1,
        650,
        48,
        72,
        16,
        '[{"name":"chicken breast","quantity":170,"unit":"g","category":"protein"},{"name":"jasmine rice","quantity":150,"unit":"g","category":"grains"},{"name":"broccoli","quantity":120,"unit":"g","category":"produce"}]'::jsonb,
        '["Cook chicken and rice.","Assemble bowl with vegetables."]'::jsonb,
        NOW(),
        NOW()
    ),
    (
        '50000000-0000-0000-0000-000000000004'::uuid,
        'turkey-wrap',
        'Turkey Wrap',
        'lunch',
        'Turkey wrap with hummus and greens.',
        1,
        610,
        42,
        60,
        18,
        '[{"name":"whole wheat wrap","quantity":1,"unit":"item","category":"grains"},{"name":"turkey breast","quantity":140,"unit":"g","category":"protein"},{"name":"hummus","quantity":45,"unit":"g","category":"fats"},{"name":"mixed greens","quantity":60,"unit":"g","category":"produce"}]'::jsonb,
        '["Layer ingredients in wrap.","Roll and slice."]'::jsonb,
        NOW(),
        NOW()
    ),
    (
        '50000000-0000-0000-0000-000000000005'::uuid,
        'salmon-potato-plate',
        'Salmon Potato Plate',
        'dinner',
        'Roasted salmon with potatoes and green beans.',
        1,
        700,
        46,
        54,
        28,
        '[{"name":"salmon fillet","quantity":180,"unit":"g","category":"protein"},{"name":"potatoes","quantity":250,"unit":"g","category":"produce"},{"name":"green beans","quantity":140,"unit":"g","category":"produce"}]'::jsonb,
        '["Roast salmon and potatoes.","Steam green beans and plate."]'::jsonb,
        NOW(),
        NOW()
    ),
    (
        '50000000-0000-0000-0000-000000000006'::uuid,
        'lean-beef-pasta',
        'Lean Beef Pasta',
        'dinner',
        'Lean beef with pasta and tomato sauce.',
        1,
        720,
        50,
        78,
        20,
        '[{"name":"lean ground beef","quantity":170,"unit":"g","category":"protein"},{"name":"dry pasta","quantity":90,"unit":"g","category":"grains"},{"name":"tomato sauce","quantity":180,"unit":"g","category":"produce"}]'::jsonb,
        '["Cook pasta.","Brown beef and add sauce.","Combine and serve."]'::jsonb,
        NOW(),
        NOW()
    )
ON CONFLICT (slug)
DO UPDATE SET
    name = EXCLUDED.name,
    meal_type = EXCLUDED.meal_type,
    description = EXCLUDED.description,
    servings = EXCLUDED.servings,
    calories_kcal = EXCLUDED.calories_kcal,
    protein_g = EXCLUDED.protein_g,
    carbs_g = EXCLUDED.carbs_g,
    fat_g = EXCLUDED.fat_g,
    ingredients_json = EXCLUDED.ingredients_json,
    instructions_json = EXCLUDED.instructions_json,
    updated_at = NOW();
