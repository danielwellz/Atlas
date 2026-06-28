import { useMutation } from '@tanstack/react-query';
import type { components } from '../../api/generated/openapi';
import { loginUser, registerUser } from '../../api/services/authService';
import { useAuth } from '../../state/AuthContext';
import { useMockMode } from '../../state/MockModeContext';

type LoginRequest = components['schemas']['LoginRequest'];
type RegisterRequest = components['schemas']['RegisterRequest'];

export function useLoginMutation() {
  const { isMockMode } = useMockMode();
  const { applyAuthResponse } = useAuth();

  return useMutation({
    mutationFn: (payload: LoginRequest) => loginUser(payload, isMockMode),
    onSuccess: async response => {
      await applyAuthResponse(response);
    },
  });
}

export function useRegisterMutation() {
  const { isMockMode } = useMockMode();
  const { applyAuthResponse } = useAuth();

  return useMutation({
    mutationFn: (payload: RegisterRequest) => registerUser(payload, isMockMode),
    onSuccess: async response => {
      await applyAuthResponse(response);
    },
  });
}
