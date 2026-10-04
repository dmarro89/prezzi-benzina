import { useCallback, useEffect, useState } from 'react';
import Ionicons from '@expo/vector-icons/Ionicons';
import Constants from 'expo-constants';
import * as Location from 'expo-location';
import {
  ActivityIndicator,
  Alert,
  FlatList,
  Linking,
  Modal,
  Platform,
  Pressable,
  RefreshControl,
  SafeAreaView,
  StatusBar,
  StyleSheet,
  Text,
  View,
} from 'react-native';

type Fuel = 'benzina' | 'gasolio' | 'gpl' | 'metano';
type Sort = 'price' | 'distance';
type Service = 'self' | 'served';
type Radius = 2 | 5 | 10;

type Price = { fuel: string; value: number; unit: string; service: string; updatedAt?: string };
type Station = {
  id: number;
  brand: string;
  name: string;
  address: string;
  city: string;
  province: string;
  location: { lat: number; lng: number };
  distanceKm: number;
  price: Price;
  prices?: Price[];
};

type NearbyResponse = { dataSource: string; liveData?: boolean; count: number; stations: Station[] };

const palette = {
  background: '#F4F4F0',
  surface: '#FFFFFF',
  text: '#171916',
  muted: '#737770',
  subtle: '#A6AAA3',
  line: '#DDDCD5',
  accent: '#176B4D',
  accentSoft: '#E2EEE8',
  error: '#9B2C2C',
};

function resolveApiUrl() {
  const configured = process.env.EXPO_PUBLIC_API_URL?.trim();
  if (configured) return configured.replace(/\/+$/, '');

  const hostUri = Constants.expoConfig?.hostUri;
  if (__DEV__ && hostUri) {
    try {
      const parsed = new URL(hostUri.includes('://') ? hostUri : `http://${hostUri}`);
      const host = parsed.hostname;
      const isLocalHost =
        host === 'localhost' ||
        host.endsWith('.local') ||
        /^10\./.test(host) ||
        /^192\.168\./.test(host) ||
        /^172\.(1[6-9]|2\d|3[01])\./.test(host) ||
        /^169\.254\./.test(host);
      if (isLocalHost) return `http://${host}:8080`;
    } catch {
      // Simulators can keep using localhost; physical devices should use EXPO_PUBLIC_API_URL.
    }
  }

  return 'http://localhost:8080';
}

const API_URL = resolveApiUrl();
const fuels: { key: Fuel; label: string }[] = [
  { key: 'benzina', label: 'Benzina' },
  { key: 'gasolio', label: 'Diesel' },
  { key: 'gpl', label: 'GPL' },
  { key: 'metano', label: 'Metano' },
];
const radii: Radius[] = [2, 5, 10];

export default function App() {
  const [fuel, setFuel] = useState<Fuel>('benzina');
  const [service, setService] = useState<Service>('self');
  const [radius, setRadius] = useState<Radius>(5);
  const [sort, setSort] = useState<Sort>('price');
  const [stations, setStations] = useState<Station[]>([]);
  const [coords, setCoords] = useState<{ lat: number; lng: number } | null>(null);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [showInfo, setShowInfo] = useState(false);

  const findLocation = useCallback(async () => {
    try {
      const permission = await Location.requestForegroundPermissionsAsync();
      if (permission.status !== 'granted') throw new Error('Permesso posizione non concesso');

      const last = await Location.getLastKnownPositionAsync({ maxAge: 10 * 60_000, requiredAccuracy: 3000 });
      const current = last ?? (await Location.getCurrentPositionAsync({ accuracy: Location.Accuracy.Balanced }));
      const next = { lat: current.coords.latitude, lng: current.coords.longitude };
      setCoords(next);
      return next;
    } catch (e) {
      const message = e instanceof Error ? e.message : 'errore sconosciuto';
      throw new Error(`Localizzazione: ${message}`);
    }
  }, []);

  const load = useCallback(async (isRefresh = false) => {
    try {
      isRefresh ? setRefreshing(true) : setLoading(true);
      setError(null);
      const location = coords ?? (await findLocation());
      const params = new URLSearchParams({
        lat: String(location.lat),
        lng: String(location.lng),
        radiusKm: String(radius),
        fuel,
        service,
        sort,
      });

      const controller = new AbortController();
      const timeout = setTimeout(() => controller.abort(), 10_000);
      let response: Response;
      try {
        response = await fetch(`${API_URL}/v1/stations/nearby?${params}`, { signal: controller.signal });
      } catch (e) {
        const message = e instanceof Error ? e.message : 'errore sconosciuto';
        if (controller.signal.aborted) throw new Error('Il caricamento ha superato i 10 secondi');
        throw new Error(`Backend non raggiungibile: ${message}`);
      } finally {
        clearTimeout(timeout);
      }

      if (!response.ok) {
        const body = await response.text();
        throw new Error(`API ${response.status}: ${body}`);
      }

      const data: NearbyResponse = await response.json();
      setStations(data.stations);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Errore sconosciuto');
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  }, [coords, findLocation, fuel, service, radius, sort]);

  useEffect(() => {
    void load();
  }, [fuel, service, radius, sort]);

  return (
    <SafeAreaView style={styles.safe}>
      <StatusBar barStyle="dark-content" />
      <FlatList
        data={stations}
        keyExtractor={(item) => String(item.id)}
        refreshControl={
          <RefreshControl
            refreshing={refreshing}
            onRefresh={() => void load(true)}
            tintColor={palette.accent}
          />
        }
        contentContainerStyle={styles.content}
        ListHeaderComponent={
          <View>
            <View style={styles.header}>
              <View style={styles.headerCopy}>
                <Text style={styles.appTitle}>Prezzi carburante</Text>
                <View style={styles.locationRow}>
                  <Ionicons name="location-outline" size={14} color={palette.muted} />
                  <Text style={styles.locationText}>
                    {coords ? `Entro ${radius} km dalla tua posizione` : 'Sto cercando la tua posizione'}
                  </Text>
                </View>
              </View>
              <View style={styles.headerActions}>
                <Pressable
                  accessibilityLabel="Informazioni"
                  onPress={() => setShowInfo(true)}
                  style={({ pressed }) => [styles.iconButton, pressed && styles.pressed]}
                >
                  <Ionicons name="information-outline" size={20} color={palette.text} />
                </Pressable>
                <Pressable
                  accessibilityLabel="Aggiorna"
                  onPress={() => void load(true)}
                  style={({ pressed }) => [styles.iconButton, pressed && styles.pressed]}
                >
                  <Ionicons name="refresh" size={20} color={palette.text} />
                </Pressable>
              </View>
            </View>

            <View style={styles.sourceRow}>
              <View style={styles.liveDot} />
              <Text style={styles.sourceText}>Prezzi ufficiali MIMIT</Text>
            </View>

            <View style={styles.fuelTabs}>
              {fuels.map((item) => {
                const active = fuel === item.key;
                return (
                  <Pressable
                    key={item.key}
                    onPress={() => setFuel(item.key)}
                    style={[styles.fuelTab, active && styles.fuelTabActive]}
                  >
                    <Text style={[styles.fuelTabText, active && styles.fuelTabTextActive]}>{item.label}</Text>
                  </Pressable>
                );
              })}
            </View>

            <View style={styles.filtersPanel}>
              <View style={styles.filterRow}>
                <Text style={styles.filterTitle}>Servizio</Text>
                <View style={styles.optionRow}>
                  {(['self', 'served'] as Service[]).map((item) => {
                    const active = service === item;
                    return (
                      <Pressable
                        key={item}
                        onPress={() => setService(item)}
                        style={[styles.optionButton, active && styles.optionButtonActive]}
                      >
                        <Text style={[styles.optionText, active && styles.optionTextActive]}>
                          {item === 'self' ? 'Self' : 'Servito'}
                        </Text>
                      </Pressable>
                    );
                  })}
                </View>
              </View>

              <View style={styles.filterDivider} />

              <View style={styles.filterRow}>
                <Text style={styles.filterTitle}>Raggio</Text>
                <View style={styles.optionRow}>
                  {radii.map((item) => {
                    const active = radius === item;
                    return (
                      <Pressable
                        key={item}
                        onPress={() => setRadius(item)}
                        style={[styles.optionButton, active && styles.optionButtonActive]}
                      >
                        <Text style={[styles.optionText, active && styles.optionTextActive]}>{item} km</Text>
                      </Pressable>
                    );
                  })}
                </View>
              </View>
            </View>

            <View style={styles.resultsHeader}>
              <Text style={styles.resultsCount}>
                {loading ? 'Distributori' : `${stations.length} distributori`}
              </Text>
              <Pressable
                onPress={() => setSort((value) => (value === 'price' ? 'distance' : 'price'))}
                style={({ pressed }) => [styles.sortControl, pressed && styles.pressed]}
              >
                <Ionicons name="swap-vertical-outline" size={15} color={palette.text} />
                <Text style={styles.sortText}>{sort === 'price' ? 'Prezzo' : 'Distanza'}</Text>
              </Pressable>
            </View>

            {loading ? (
              <View style={styles.loading}>
                <ActivityIndicator color={palette.accent} />
                <Text style={styles.loadingText}>Aggiorno i prezzi…</Text>
              </View>
            ) : null}

            {error ? (
              <View style={styles.errorBox}>
                <Ionicons name="alert-circle-outline" size={20} color={palette.error} />
                <View style={styles.errorCopy}>
                  <Text style={styles.errorTitle}>Impossibile caricare i distributori</Text>
                  <Text style={styles.errorText}>{error}</Text>
                  <Pressable onPress={() => void load()} style={styles.retryButton}>
                    <Text style={styles.retryText}>Riprova</Text>
                  </Pressable>
                </View>
              </View>
            ) : null}
          </View>
        }
        renderItem={({ item, index }) => (
          <StationRow station={item} best={sort === 'price' && index === 0} />
        )}
        ListEmptyComponent={
          !loading && !error ? <Text style={styles.empty}>Nessun distributore trovato con questi filtri.</Text> : null
        }
        ListFooterComponent={
          <Text style={styles.footer}>Fonte dati: Ministero delle Imprese e del Made in Italy</Text>
        }
      />
      <InfoModal visible={showInfo} onClose={() => setShowInfo(false)} />
    </SafeAreaView>
  );
}

function InfoModal({ visible, onClose }: { visible: boolean; onClose: () => void }) {
  const version = Constants.expoConfig?.version ?? '0.1.0';

  return (
    <Modal visible={visible} animationType="slide" presentationStyle="pageSheet" onRequestClose={onClose}>
      <SafeAreaView style={styles.infoSafe}>
        <View style={styles.infoHeader}>
          <Text style={styles.infoTitle}>Informazioni</Text>
          <Pressable accessibilityLabel="Chiudi" onPress={onClose} style={({ pressed }) => [styles.infoClose, pressed && styles.pressed]}>
            <Ionicons name="close" size={22} color={palette.text} />
          </Pressable>
        </View>

        <View style={styles.infoSection}>
          <Text style={styles.infoSectionTitle}>Dati</Text>
          <Text style={styles.infoBody}>
            I distributori e i prezzi arrivano da Osservaprezzi carburanti e dagli Open Data del Ministero delle Imprese e del Made in Italy.
          </Text>
        </View>

        <View style={styles.infoSection}>
          <Text style={styles.infoSectionTitle}>Aggiornamento prezzi</Text>
          <Text style={styles.infoBody}>
            Mostriamo solo prezzi comunicati negli ultimi 8 giorni. Data e ora accanto al prezzo indicano l'ultimo aggiornamento disponibile per quel servizio.
          </Text>
        </View>

        <View style={styles.infoSection}>
          <Text style={styles.infoSectionTitle}>Posizione</Text>
          <Text style={styles.infoBody}>
            La posizione serve esclusivamente a trovare i distributori vicini. Non viene salvata dal servizio e l'app non usa sistemi di analytics o profilazione.
          </Text>
        </View>

        <View style={styles.infoSection}>
          <Text style={styles.infoSectionTitle}>Coordinate</Text>
          <Text style={styles.infoBody}>
            Distanze e navigazione dipendono dalle coordinate pubblicate dalle fonti ufficiali; eventuali imprecisioni possono riflettersi sulla posizione mostrata.
          </Text>
        </View>

        <Text style={styles.infoVersion}>Prezzi Benzina · v{version}</Text>
      </SafeAreaView>
    </Modal>
  );
}

function StationRow({ station, best }: { station: Station; best: boolean }) {
  const navigate = () => {
    const { lat, lng } = station.location;
    if (!Number.isFinite(lat) || !Number.isFinite(lng) || lat < -90 || lat > 90 || lng < -180 || lng > 180) {
      Alert.alert('Posizione non disponibile', 'Le coordinate di questo distributore non sono valide.');
      return;
    }

    const destination = encodeURIComponent(`${lat},${lng}`);
    const url =
      Platform.OS === 'ios'
        ? `https://maps.apple.com/directions?destination=${destination}`
        : `https://www.google.com/maps/dir/?api=1&destination=${destination}&travelmode=driving`;
    void Linking.openURL(url);
  };

  const name = (station.name || station.brand || 'Distributore').trim();
  const brand = station.brand?.trim() ?? '';
  const showBrand = brand !== '' && brand.toLocaleLowerCase('it-IT') !== name.toLocaleLowerCase('it-IT');
  const address = station.address || 'Indirizzo non disponibile';
  const locality = station.city
    ? `${station.city}${station.province ? ` (${station.province})` : ''}`
    : station.province;
  const updated = station.price.updatedAt ? shortUpdated(station.price.updatedAt) : null;
  const unit = station.price.unit === 'EUR/kg' ? '€/kg' : '€/L';

  return (
    <Pressable onPress={navigate} style={({ pressed }) => [styles.stationRow, pressed && styles.stationPressed]}>
      <View style={styles.stationMain}>
        {best ? (
          <View style={styles.bestRow}>
            <View style={styles.bestDot} />
            <Text style={styles.bestLabel}>Prezzo più basso</Text>
          </View>
        ) : null}

        <Text numberOfLines={1} style={styles.stationName}>{name}</Text>
        {showBrand ? <Text numberOfLines={1} style={styles.brandLabel}>{brand}</Text> : null}
        <Text numberOfLines={1} style={styles.address}>{address}</Text>
        {locality ? <Text numberOfLines={1} style={styles.locality}>{locality}</Text> : null}

        <View style={styles.metaRow}>
          <Ionicons name="navigate-outline" size={13} color={palette.muted} />
          <Text style={styles.metaText}>{station.distanceKm.toFixed(1).replace('.', ',')} km</Text>
          {updated ? (
            <>
              <Text style={styles.metaSeparator}>·</Text>
              <Ionicons name="time-outline" size={13} color={palette.muted} />
              <Text style={styles.metaText}>{updated}</Text>
            </>
          ) : null}
        </View>
      </View>

      <View style={styles.priceColumn}>
        <Text style={[styles.price, best && styles.priceBest]}>{formatPrice(station.price.value)}</Text>
        <Text style={styles.priceMeta}>{unit} · {station.price.service === 'self' ? 'Self' : 'Servito'}</Text>
        <Ionicons name="chevron-forward" size={17} color={palette.subtle} style={styles.chevron} />
      </View>
    </Pressable>
  );
}

function formatPrice(value: number) {
  return value.toFixed(3).replace('.', ',');
}

function shortUpdated(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return 'aggiornato';

  const now = new Date();
  const dateDay = Date.UTC(date.getFullYear(), date.getMonth(), date.getDate());
  const today = Date.UTC(now.getFullYear(), now.getMonth(), now.getDate());
  const dayDiff = Math.round((today - dateDay) / 86_400_000);
  const time = date.toLocaleTimeString('it-IT', { hour: '2-digit', minute: '2-digit' });

  if (dayDiff === 0) return `oggi ${time}`;
  if (dayDiff === 1) return `ieri ${time}`;

  const day = date.toLocaleDateString('it-IT', { day: '2-digit', month: '2-digit' });
  return `${day} ${time}`;
}

const displayFont = Platform.select({ ios: 'Avenir Next', default: undefined });

const styles = StyleSheet.create({
  safe: { flex: 1, backgroundColor: palette.background },
  content: { paddingHorizontal: 20, paddingTop: 12, paddingBottom: 32 },

  header: { flexDirection: 'row', alignItems: 'flex-start', justifyContent: 'space-between', marginTop: 8 },
  headerCopy: { flex: 1, paddingRight: 16 },
  headerActions: { flexDirection: 'row', gap: 8 },
  appTitle: {
    color: palette.text,
    fontSize: 30,
    lineHeight: 36,
    fontWeight: '700',
    letterSpacing: -0.8,
    fontFamily: displayFont,
  },
  locationRow: { flexDirection: 'row', alignItems: 'center', gap: 5, marginTop: 6 },
  locationText: { color: palette.muted, fontSize: 13 },
  iconButton: {
    width: 40,
    height: 40,
    borderRadius: 20,
    borderWidth: 1,
    borderColor: palette.line,
    backgroundColor: palette.surface,
    alignItems: 'center',
    justifyContent: 'center',
  },
  pressed: { opacity: 0.55 },

  sourceRow: { flexDirection: 'row', alignItems: 'center', gap: 7, marginTop: 18 },
  liveDot: { width: 7, height: 7, borderRadius: 4, backgroundColor: palette.accent },
  sourceText: { color: palette.muted, fontSize: 12, fontWeight: '600' },

  fuelTabs: {
    flexDirection: 'row',
    marginTop: 30,
    borderBottomWidth: 1,
    borderBottomColor: palette.line,
  },
  fuelTab: { flex: 1, paddingBottom: 12, alignItems: 'center', borderBottomWidth: 2, borderBottomColor: 'transparent' },
  fuelTabActive: { borderBottomColor: palette.text },
  fuelTabText: { color: palette.subtle, fontSize: 13, fontWeight: '600' },
  fuelTabTextActive: { color: palette.text },

  filtersPanel: {
    marginTop: 18,
    backgroundColor: palette.surface,
    borderRadius: 14,
    paddingHorizontal: 14,
    borderWidth: 1,
    borderColor: palette.line,
  },
  filterRow: { minHeight: 58, flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', gap: 12 },
  filterTitle: { color: palette.text, fontSize: 13, fontWeight: '600' },
  filterDivider: { height: 1, backgroundColor: palette.line },
  optionRow: { flexDirection: 'row', gap: 5 },
  optionButton: { paddingVertical: 7, paddingHorizontal: 11, borderRadius: 8 },
  optionButtonActive: { backgroundColor: palette.text },
  optionText: { color: palette.muted, fontSize: 12, fontWeight: '600' },
  optionTextActive: { color: '#FFFFFF' },

  resultsHeader: {
    marginTop: 28,
    marginBottom: 4,
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
  },
  resultsCount: {
    color: palette.text,
    fontSize: 19,
    fontWeight: '700',
    letterSpacing: -0.25,
    fontFamily: displayFont,
  },
  sortControl: { flexDirection: 'row', alignItems: 'center', gap: 5, paddingVertical: 7, paddingLeft: 10 },
  sortText: { color: palette.text, fontSize: 12, fontWeight: '600' },

  loading: { paddingVertical: 34, alignItems: 'center', gap: 10 },
  loadingText: { color: palette.muted, fontSize: 12 },
  errorBox: {
    marginTop: 16,
    flexDirection: 'row',
    gap: 10,
    borderWidth: 1,
    borderColor: '#E5CACA',
    backgroundColor: '#FFF7F7',
    borderRadius: 12,
    padding: 14,
  },
  errorCopy: { flex: 1 },
  errorTitle: { color: palette.error, fontWeight: '700', fontSize: 13 },
  errorText: { color: '#765555', fontSize: 11, lineHeight: 16, marginTop: 4 },
  retryButton: { alignSelf: 'flex-start', marginTop: 10, paddingVertical: 4 },
  retryText: { color: palette.text, fontSize: 12, fontWeight: '700' },

  stationRow: {
    minHeight: 128,
    flexDirection: 'row',
    alignItems: 'center',
    borderBottomWidth: 1,
    borderBottomColor: palette.line,
    paddingVertical: 16,
  },
  stationPressed: { opacity: 0.55 },
  stationMain: { flex: 1, minWidth: 0, paddingRight: 12 },
  bestRow: { flexDirection: 'row', alignItems: 'center', gap: 6, marginBottom: 5 },
  bestDot: { width: 6, height: 6, borderRadius: 3, backgroundColor: palette.accent },
  bestLabel: { color: palette.accent, fontSize: 10, fontWeight: '700' },
  stationName: {
    color: palette.text,
    fontSize: 16,
    lineHeight: 21,
    fontWeight: '700',
    letterSpacing: -0.2,
    fontFamily: displayFont,
  },
  brandLabel: { color: palette.muted, fontSize: 11, fontWeight: '600', marginTop: 2 },
  address: { color: '#4D514C', fontSize: 12, marginTop: 7 },
  locality: { color: palette.muted, fontSize: 11, marginTop: 2 },
  metaRow: { flexDirection: 'row', alignItems: 'center', gap: 4, marginTop: 9 },
  metaText: { color: palette.muted, fontSize: 10.5 },
  metaSeparator: { color: palette.subtle, fontSize: 11, marginHorizontal: 1 },

  priceColumn: { minWidth: 88, alignItems: 'flex-end' },
  price: {
    color: palette.text,
    fontSize: 25,
    lineHeight: 30,
    fontWeight: '700',
    letterSpacing: -0.8,
    fontVariant: ['tabular-nums'],
    fontFamily: displayFont,
  },
  priceBest: { color: palette.accent },
  priceMeta: { color: palette.muted, fontSize: 10, marginTop: 1 },
  chevron: { marginTop: 14 },

  empty: { color: palette.muted, paddingVertical: 42, textAlign: 'center', fontSize: 13 },
  footer: { color: palette.subtle, fontSize: 10, textAlign: 'center', marginTop: 24, lineHeight: 15 },

  infoSafe: { flex: 1, backgroundColor: palette.background, paddingHorizontal: 22 },
  infoHeader: { flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', paddingTop: 12, paddingBottom: 24 },
  infoTitle: { color: palette.text, fontSize: 28, fontWeight: '700', letterSpacing: -0.6, fontFamily: displayFont },
  infoClose: { width: 40, height: 40, borderRadius: 20, borderWidth: 1, borderColor: palette.line, backgroundColor: palette.surface, alignItems: 'center', justifyContent: 'center' },
  infoSection: { paddingVertical: 18, borderBottomWidth: 1, borderBottomColor: palette.line },
  infoSectionTitle: { color: palette.text, fontSize: 14, fontWeight: '700', marginBottom: 7 },
  infoBody: { color: '#555A54', fontSize: 13, lineHeight: 20 },
  infoVersion: { color: palette.subtle, fontSize: 11, marginTop: 24 },
});
