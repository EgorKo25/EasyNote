import { Platform, StyleSheet } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { ThemedText } from '@/components/themed-text';
import { ThemedView } from '@/components/themed-view';
import { MaxContentWidth, Spacing } from '@/constants/theme';

const platformLabel =
  Platform.OS === 'web' ? 'Web (Telegram Mini App target)' : `Native (${Platform.OS})`;

export default function HomeScreen() {
  return (
    <ThemedView style={styles.container}>
      <SafeAreaView style={styles.safeArea}>
        <ThemedView style={styles.content}>
          <ThemedText type="title" style={styles.title}>
            EasyNote Frontend
          </ThemedText>
          <ThemedText type="default" style={styles.subtitle}>
            Reminder app for Telegram Mini App and mobile platforms.
          </ThemedText>
          <ThemedView type="backgroundElement" style={styles.meta}>
            <ThemedText type="small">Platform: {platformLabel}</ThemedText>
            <ThemedText type="small">Backend API integration is planned — see docs/roadmap.md</ThemedText>
          </ThemedView>
        </ThemedView>
      </SafeAreaView>
    </ThemedView>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    justifyContent: 'center',
    flexDirection: 'row',
  },
  safeArea: {
    flex: 1,
    paddingHorizontal: Spacing.four,
    alignItems: 'center',
    justifyContent: 'center',
    maxWidth: MaxContentWidth,
  },
  content: {
    alignItems: 'center',
    gap: Spacing.three,
    paddingHorizontal: Spacing.four,
  },
  title: {
    textAlign: 'center',
  },
  subtitle: {
    textAlign: 'center',
  },
  meta: {
    alignSelf: 'stretch',
    gap: Spacing.two,
    paddingHorizontal: Spacing.three,
    paddingVertical: Spacing.three,
    borderRadius: Spacing.three,
    marginTop: Spacing.two,
  },
});
