package usage

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcontact "github.com/larksuite/oapi-sdk-go/v3/service/contact/v3"

	"lab/askplanner/internal/config"
)

const (
	usageCLIUserName         = "CLI"
	usagePositiveCacheTTL    = 6 * time.Hour
	usageNegativeCacheTTL    = 10 * time.Minute
	usageWildcardBotCacheKey = "*"
)

type usageUserResolver interface {
	Resolve(ctx context.Context, source, userKey, conversationKey string) string
}

type usageResolverBot struct {
	key    string
	client usageUserLookup
}

type usageUserLookup interface {
	LookupUserName(ctx context.Context, rawID, idType string) string
}

type feishuContactLookup struct {
	client *larkcontact.V3
}

type cachedUsageUserName struct {
	name      string
	expiresAt time.Time
}

type feishuUserNameResolver struct {
	bots  []usageResolverBot
	now   func() time.Time
	mu    sync.Mutex
	cache map[string]cachedUsageUserName
}

type usageUserIdentity struct {
	botKey string
	rawID  string
}

func newUsageUserResolver(cfg *config.Config) usageUserResolver {
	if cfg == nil {
		log.Printf("[usage] user resolver disabled: config is nil")
		return nil
	}
	bots := make([]usageResolverBot, 0, len(cfg.LarkBots))
	skipped := 0
	for _, botCfg := range cfg.LarkBots {
		if strings.TrimSpace(botCfg.AppID) == "" || strings.TrimSpace(botCfg.AppSecret) == "" {
			skipped++
			continue
		}
		client := lark.NewClient(botCfg.AppID, botCfg.AppSecret)
		bots = append(bots, usageResolverBot{
			key:    strings.TrimSpace(botCfg.Key),
			client: feishuContactLookup{client: client.Contact.V3},
		})
	}
	if len(bots) == 0 {
		log.Printf("[usage] user resolver disabled: no valid Feishu bot credentials found (configured=%d skipped=%d)", len(cfg.LarkBots), skipped)
		return nil
	}
	keys := make([]string, 0, len(bots))
	for _, bot := range bots {
		keys = append(keys, fallbackString(bot.key, "<default>"))
	}
	log.Printf("[usage] user resolver enabled: bots=%d keys=%s", len(bots), strings.Join(keys, ","))
	return &feishuUserNameResolver{
		bots:  bots,
		now:   time.Now,
		cache: make(map[string]cachedUsageUserName),
	}
}

func (r *feishuUserNameResolver) Resolve(ctx context.Context, source, userKey, conversationKey string) string {
	if r == nil {
		return ""
	}
	if normalizeSource(source) == sourceCLI || strings.TrimSpace(userKey) == cliVirtualUserKey {
		return usageCLIUserName
	}
	identity := parseUsageUserIdentity(userKey, conversationKey)
	if identity.rawID == "" {
		log.Printf("[usage] user resolver skipped: source=%s user_key=%s conversation=%s reason=no_resolvable_user_id",
			strings.TrimSpace(source), compactLogField(userKey, 80), compactLogField(conversationKey, 120))
		return ""
	}
	cacheKey := usageResolverCacheKey(identity)
	if name, ok := r.lookupCache(cacheKey); ok {
		log.Printf("[usage] user resolver cache hit: bot=%s raw_id=%s name=%s",
			fallbackString(identity.botKey, usageWildcardBotCacheKey), compactLogField(identity.rawID, 64), compactLogField(name, 64))
		return name
	}
	name := r.lookupRemote(ctx, identity)
	r.storeCache(cacheKey, name)
	if strings.TrimSpace(name) == "" {
		log.Printf("[usage] user resolver lookup failed: bot=%s raw_id=%s user_key=%s conversation=%s",
			fallbackString(identity.botKey, usageWildcardBotCacheKey),
			compactLogField(identity.rawID, 64),
			compactLogField(userKey, 80),
			compactLogField(conversationKey, 120))
	} else {
		log.Printf("[usage] user resolver lookup success: bot=%s raw_id=%s name=%s",
			fallbackString(identity.botKey, usageWildcardBotCacheKey),
			compactLogField(identity.rawID, 64),
			compactLogField(name, 64))
	}
	return name
}

func (r *feishuUserNameResolver) lookupRemote(ctx context.Context, identity usageUserIdentity) string {
	bots := r.selectBots(identity.botKey)
	if len(bots) == 0 {
		log.Printf("[usage] user resolver has no matching bot client: bot=%s raw_id=%s",
			fallbackString(identity.botKey, usageWildcardBotCacheKey), compactLogField(identity.rawID, 64))
		return ""
	}
	for _, bot := range bots {
		if name := lookupFeishuName(ctx, bot.client, identity.rawID, larkcontact.UserIdTypeGetUserOpenId); name != "" {
			return name
		}
		if name := lookupFeishuName(ctx, bot.client, identity.rawID, larkcontact.UserIdTypeGetUserUserId); name != "" {
			return name
		}
	}
	return ""
}

func (r *feishuUserNameResolver) selectBots(botKey string) []usageResolverBot {
	botKey = strings.TrimSpace(botKey)
	if botKey == "" {
		return r.bots
	}
	for _, bot := range r.bots {
		if bot.key == botKey {
			return []usageResolverBot{bot}
		}
	}
	return nil
}

func (r *feishuUserNameResolver) lookupCache(key string) (string, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	item, ok := r.cache[key]
	if !ok {
		return "", false
	}
	now := r.now()
	if now.After(item.expiresAt) {
		delete(r.cache, key)
		return "", false
	}
	return item.name, true
}

func (r *feishuUserNameResolver) storeCache(key, name string) {
	if key == "" {
		return
	}
	ttl := usageNegativeCacheTTL
	if strings.TrimSpace(name) != "" {
		ttl = usagePositiveCacheTTL
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cache[key] = cachedUsageUserName{
		name:      strings.TrimSpace(name),
		expiresAt: r.now().Add(ttl),
	}
}

func usageResolverCacheKey(identity usageUserIdentity) string {
	if strings.TrimSpace(identity.rawID) == "" {
		return ""
	}
	botKey := strings.TrimSpace(identity.botKey)
	if botKey == "" {
		botKey = usageWildcardBotCacheKey
	}
	return botKey + "\x00" + strings.TrimSpace(identity.rawID)
}

func parseUsageUserIdentity(userKey, conversationKey string) usageUserIdentity {
	userKey = strings.TrimSpace(userKey)
	conversationKey = strings.TrimSpace(conversationKey)

	if botKey, rawID, ok := parseScopedLarkUserKey(userKey); ok {
		return usageUserIdentity{botKey: botKey, rawID: rawID}
	}
	if botKey, rawID, ok := parseScopedConversationUserID(conversationKey); ok {
		return usageUserIdentity{botKey: botKey, rawID: rawID}
	}
	if rawID, ok := parseLegacyConversationUserID(conversationKey); ok {
		return usageUserIdentity{rawID: rawID}
	}
	if userKey != "" && userKey != cliVirtualUserKey {
		return usageUserIdentity{botKey: parseConversationBotKey(conversationKey), rawID: userKey}
	}
	return usageUserIdentity{}
}

func parseScopedLarkUserKey(userKey string) (string, string, bool) {
	parts := strings.SplitN(strings.TrimSpace(userKey), ":", 3)
	if len(parts) != 3 || parts[0] != "larkbot" {
		return "", "", false
	}
	if strings.TrimSpace(parts[1]) == "" || strings.TrimSpace(parts[2]) == "" {
		return "", "", false
	}
	return parts[1], parts[2], true
}

func parseConversationBotKey(conversationKey string) string {
	parts := strings.SplitN(strings.TrimSpace(conversationKey), ":", 3)
	if len(parts) != 3 || parts[0] != "larkbot" {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func parseLegacyConversationUserID(conversationKey string) (string, bool) {
	idx := strings.LastIndex(strings.TrimSpace(conversationKey), ":user:")
	if idx < 0 {
		return "", false
	}
	rawID := strings.TrimSpace(conversationKey[idx+len(":user:"):])
	if rawID == "" {
		return "", false
	}
	return rawID, true
}

func parseScopedConversationUserID(conversationKey string) (string, string, bool) {
	botKey := parseConversationBotKey(conversationKey)
	if botKey == "" {
		return "", "", false
	}
	idx := strings.LastIndex(strings.TrimSpace(conversationKey), ":user:")
	if idx < 0 {
		return "", "", false
	}
	userSegment := strings.TrimSpace(conversationKey[idx+len(":user:"):])
	prefix := "larkbot_" + botKey + "_"
	if !strings.HasPrefix(userSegment, prefix) {
		return "", "", false
	}
	rawID := strings.TrimPrefix(userSegment, prefix)
	if rawID == "" {
		return "", "", false
	}
	return botKey, rawID, true
}

func lookupFeishuName(ctx context.Context, client usageUserLookup, rawID, idType string) string {
	if client == nil || strings.TrimSpace(rawID) == "" {
		return ""
	}
	return client.LookupUserName(ctx, rawID, idType)
}

func (l feishuContactLookup) LookupUserName(ctx context.Context, rawID, idType string) string {
	if l.client == nil || l.client.User == nil || strings.TrimSpace(rawID) == "" {
		return ""
	}
	req := larkcontact.NewGetUserReqBuilder().
		UserId(strings.TrimSpace(rawID)).
		UserIdType(idType).
	Build()
	resp, err := l.client.User.Get(ctx, req)
	if err != nil || resp == nil || !resp.Success() || resp.Data == nil || resp.Data.User == nil {
		if err != nil {
			log.Printf("[usage] feishu lookup error: raw_id=%s id_type=%s err=%v", rawID, idType, err)
		} else if resp != nil {
			log.Printf("[usage] feishu lookup miss: raw_id=%s id_type=%s code=%d msg=%s",
				rawID, idType, resp.Code, strings.TrimSpace(resp.Msg))
		}
		return ""
	}
	if resp.Data.User.Name != nil && strings.TrimSpace(*resp.Data.User.Name) != "" {
		return strings.TrimSpace(*resp.Data.User.Name)
	}
	if resp.Data.User.EnName != nil && strings.TrimSpace(*resp.Data.User.EnName) != "" {
		return strings.TrimSpace(*resp.Data.User.EnName)
	}
	return ""
}

func compactLogField(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 || len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}
