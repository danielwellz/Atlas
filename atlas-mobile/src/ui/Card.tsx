import React from 'react';
import { StyleSheet, View, type StyleProp, type ViewStyle } from 'react-native';

type CardProps = {
  children: React.ReactNode;
  style?: StyleProp<ViewStyle>;
  testID?: string;
};

export function Card({ children, style, testID }: CardProps): React.JSX.Element {
  return (
    <View style={[styles.card, style]} testID={testID}>
      {children}
    </View>
  );
}

const styles = StyleSheet.create({
  card: {
    borderRadius: 14,
    backgroundColor: '#ffffff',
    borderWidth: 1,
    borderColor: '#e2e8f0',
    padding: 16,
    gap: 12,
  },
});
