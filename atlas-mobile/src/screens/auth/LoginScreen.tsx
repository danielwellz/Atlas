import React, { useMemo, useState } from 'react';
import { KeyboardAvoidingView, Platform, StyleSheet, Text, View } from 'react-native';
import type { NativeStackScreenProps } from '@react-navigation/native-stack';
import { useLoginMutation } from '../../features/auth/hooks';
import { Button, Card, Input } from '../../ui';
import type { AuthStackParamList } from '../../navigation/types';

type Props = NativeStackScreenProps<AuthStackParamList, 'Login'>;

export function LoginScreen({ navigation }: Props): React.JSX.Element {
  const loginMutation = useLoginMutation();
  const [email, setEmail] = useState('athlete@atlas.local');
  const [password, setPassword] = useState('atlas1234');

  const formError = useMemo(() => {
    if (!email.trim()) {
      return 'Email is required.';
    }

    if (!password.trim()) {
      return 'Password is required.';
    }

    return null;
  }, [email, password]);

  const submit = () => {
    if (formError) {
      return;
    }

    loginMutation.mutate({
      email: email.trim().toLowerCase(),
      password,
    });
  };

  return (
    <KeyboardAvoidingView
      behavior={Platform.OS === 'ios' ? 'padding' : undefined}
      style={styles.container}
      testID="login-screen">
      <Card style={styles.card}>
        <Text style={styles.title} testID="login-title">
          Atlas Login
        </Text>
        <Text style={styles.subtitle}>Continue your training plan.</Text>

        <Input
          label="Email"
          value={email}
          onChangeText={setEmail}
          keyboardType="email-address"
          autoComplete="email"
          testID="login-email-input"
        />

        <Input
          label="Password"
          value={password}
          onChangeText={setPassword}
          secureTextEntry
          autoComplete="password"
          testID="login-password-input"
        />

        {loginMutation.isError ? (
          <Text style={styles.error}>{String(loginMutation.error.message)}</Text>
        ) : null}

        <View style={styles.actions}>
          <Button
            label="Login"
            onPress={submit}
            loading={loginMutation.isPending}
            disabled={Boolean(formError)}
            testID="login-submit-button"
          />
          <Button
            label="Create account"
            variant="secondary"
            onPress={() => navigation.navigate('Register')}
            testID="go-to-register-button"
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
    fontSize: 28,
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
