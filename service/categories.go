package service

type CategoryResponse struct {
    Male    []string `json:"male"`
    Female  []string `json:"female"`
    Tags    []string `json:"tags"`
    Notes   map[string]string `json:"notes"`
}

func GetCategories() CategoryResponse {
    male := []string{
        "都市高武","东方仙侠","传统玄幻","悬疑灵异","都市脑洞","玄幻脑洞","历史古代","历史脑洞","科幻末世","西幻","都市日常","都市修真","战神","赘婿","神医","武侠","军事",
    }
    female := []string{
        "宫斗宅斗","豪门总裁","年代","星光璀璨","玄幻言情","种田","现言脑洞","快穿","古言脑洞","青春甜宠","医术","职场婚恋","悬疑恋爱","双男主","双女主","民国言情","游戏体育","马甲",
    }
    tags := []string{
        "脑洞系列","重生复仇","穿书逆袭","系统流","签到/直播","末世囤物资","基建流",
    }
    notes := map[string]string{
        "最新调整": "取消奇幻仙侠，新增西幻与东方仙侠；豪门总裁整合原霸总/豪门爽文/萌宝",
        "热门组合": "男频：都市高武>玄幻脑洞>都市日常；女频：双男主>豪门总裁>宫斗宅斗",
    }
    return CategoryResponse{Male: male, Female: female, Tags: tags, Notes: notes}
}

