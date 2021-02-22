OBJLIBS	= libxscontroller libxscommon libxstypes

all : $(OBJLIBS)

libxscontroller : libxstypes
	$(MAKE) -C xscontroller $(MFLAGS)

libxscommon : libxscommon
	$(MAKE) -C xscommon $(MFLAGS)

libxstypes :
	$(MAKE) -C xstypes $(MFLAGS) libxstypes.a

clean :
	-$(MAKE) -C xscontroller $(MFLAGS) clean
	-$(MAKE) -C xscommon $(MFLAGS) clean
	-$(MAKE) -C xstypes $(MFLAGS) clean
