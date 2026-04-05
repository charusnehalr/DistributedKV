// Lightweight class-name merger (no external dep needed for this project).
export function cn(...classes: (string | undefined | false | null)[]): string {
  return classes.filter(Boolean).join(' ')
}
