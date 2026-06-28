import React, { useMemo, useState } from 'react';
import { KeyboardAvoidingView, Platform, StyleSheet, Text, View } from 'react-native';
import type { NativeStackScreenProps } from '@react-navigation/native-stack';
import { useRegisterMutation } from '../../features/auth/hooks';
import { Button, Card, Input } from '../../ui';
import type { AuthStackParamList } from '../../navigation/types';

type Props = NativeStackScreenProps<AuthStackParamList, 'Register'>;

export function RegisterScreen({ navigation }: Props): React.JSX.Element {
  const registerMutation = useRegisterMutation();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');

  const formError = useMemo(() => {
    if (!email.trim()) {
      return 'Email is required.';
    }

    if (!password.trim()) {
      return 'Password is required.';
    }

    if (password.length < 8) {
      return 'Password must be at least 8 characters.';
    }

    if (password !== confirmPassword) {
      return 'Passwords do not match.';
    }

    return null;
  }, [email, password, confirmPassword]);

  const submit = () => {
    if (formError) {
      return;
    }

    registerMutation.mutate({
      email: email.trim().toLowerCase(),
      password,
    });
  };

  return (
    <KeyboardAvoidingView
      behavior={Platform.OS === 'ios' ? 'padding' : undefined}
      style={styles.container}
      testID="register-screen">
      <Card style={styles.card}>
        <Text style={styles.title}>Create Atlas Account</Text>
        <Text style={styles.subtitle}>Set up your fitness profile in under a minute.</Text>

        <Input
          label="Email"
          value={email}
          onChangeText={setEmail}
          keyboardType="email-address"
          autoComplete="email"
          testID="register-email-input"
        />

        <Input
          label="Password"
          value={password}
          onChangeText={setPassword}
          secureTextEntry
          autoComplete="new-password"
          testID="register-password-input"
        />

        <Input
          label="Confirm password"
          value={confirmPassword}
          onChangeText={setConfirmPassword}
          secureTextEntry
          autoComplete="new-password"
          testID="register-confirm-password-input"
        />

        {formError ? <Text style={styles.error}>{formError}</Text> : null}
        {registerMutation.isError ? (
          <Text style={styles.error}>{String(registerMutation.error.message)}</Text>
        ) : null}

        <View style={styles.actions}>
          <Button
            label="Register"
            onPress={submit}
            loading={registerMutation.isPending}
            disabled={Boolean(formError)}
            testID="register-submit-button"
          />
          <Button
            label="Back to login"
            variant="secondary"
            onPress={() => navigation.navigate('Login')}
            testID="go-to-login-button"
          />
        </View>
      </Card>
    </KeyboardAvoidingView>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    justifyContent: 'center',
    paddingHorizontal: 20,
    backgroundColor: '#f8fafc',
  },
  card: {
    gap: 14,
  },
  title: {
    fontSize: 26,
    fontWeight: '700',
    color: '#0f172a',
  },
  subtitle: {
    color: '#475569',
    marginBottom: 8,
  },
  actions: {
    gap: 10,
  },
  error: {
    color: '#b91c1c',
    fontSize: 13,
  },
});
